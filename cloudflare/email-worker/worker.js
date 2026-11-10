const INLINE_MAX_ATTEMPTS = 3;
const QUEUE_INLINE_LIMIT_BYTES = 96 * 1024;

export default {
  async fetch() {
    return new Response("BaraMail Email Worker is active.", { status: 200 });
  },

  async email(message, env, _ctx) {
    const maxBytes = parseInt(env.MAX_EMAIL_SIZE_BYTES || "5242880", 10);
    if (message.rawSize > maxBytes) {
      console.warn(JSON.stringify({
        event: "email_rejected_size",
        to: message.to,
        size: message.rawSize,
      }));
      return;
    }

    const rawBuffer = await new Response(message.raw).arrayBuffer();
    if (rawBuffer.byteLength > maxBytes) {
      return;
    }

    const receivedAt = new Date().toISOString();
    const outcome = await deliverWithInlineRetry(env, rawBuffer, message.to, message.from, receivedAt);

    if (outcome.status === "delivered" || outcome.status === "permanent_reject") {
      return;
    }

    if (outcome.status === "dlq") {
      await persistToDLQ(env, rawBuffer, message.to, message.from, receivedAt, outcome.reason);
      return;
    }

    // Transient failure after inline retries: offload to durable queue/R2 if bound, otherwise throw so MTA retries.
    const buffered = await enqueueForRetry(env, rawBuffer, message.to, message.from, receivedAt, outcome.reason);
    if (!buffered) {
      throw new Error(`Transient webhook failure (${outcome.reason}) and no durable buffer configured`);
    }
  },

  async queue(batch, env) {
    for (const msg of batch.messages) {
      const payload = msg.body;
      let rawBuffer = null;

      if (payload.r2Key && env.BARAMAIL_DLQ_BUCKET) {
        const obj = await env.BARAMAIL_DLQ_BUCKET.get(payload.r2Key);
        if (obj) {
          rawBuffer = await obj.arrayBuffer();
        }
      } else if (payload.rawBase64) {
        rawBuffer = base64ToArrayBuffer(payload.rawBase64);
      }

      if (!rawBuffer) {
        msg.ack();
        continue;
      }

      const res = await sendToBackend(env, rawBuffer, payload.envelopeTo, payload.envelopeFrom, payload.receivedAt);
      if (res.status === "delivered" || res.status === "permanent_reject") {
        if (payload.r2Key && env.BARAMAIL_DLQ_BUCKET) {
          await env.BARAMAIL_DLQ_BUCKET.delete(payload.r2Key);
        }
        msg.ack();
      } else if (res.status === "dlq") {
        await persistToDLQ(env, rawBuffer, payload.envelopeTo, payload.envelopeFrom, payload.receivedAt, res.reason);
        msg.ack();
      } else {
        msg.retry();
      }
    }
  },
};

async function deliverWithInlineRetry(env, rawBuffer, envelopeTo, envelopeFrom, receivedAt) {
  let lastOutcome = { status: "transient_error", reason: "unknown" };

  for (let attempt = 1; attempt <= INLINE_MAX_ATTEMPTS; attempt++) {
    lastOutcome = await sendToBackend(env, rawBuffer, envelopeTo, envelopeFrom, receivedAt);
    if (lastOutcome.status !== "transient_error") {
      return lastOutcome;
    }
    if (attempt < INLINE_MAX_ATTEMPTS) {
      await sleep(attempt * 600);
    }
  }
  return lastOutcome;
}

async function sendToBackend(env, rawBuffer, envelopeTo, envelopeFrom, receivedAt) {
  const headers = {
    "Content-Type": "message/rfc822",
    "X-BaraMail-Secret": env.BARAMAIL_WEBHOOK_SECRET,
    "X-BaraMail-Envelope-To": envelopeTo,
    "X-BaraMail-Envelope-From": envelopeFrom,
    "X-BaraMail-Received-At": receivedAt,
  };

  if (env.CF_ACCESS_CLIENT_ID && env.CF_ACCESS_CLIENT_SECRET) {
    headers["CF-Access-Client-Id"] = env.CF_ACCESS_CLIENT_ID;
    headers["CF-Access-Client-Secret"] = env.CF_ACCESS_CLIENT_SECRET;
  }

  try {
    const resp = await fetch(env.BARAMAIL_API_URL, {
      method: "POST",
      headers,
      body: rawBuffer,
    });

    if (resp.status === 200 || resp.status === 201) {
      return { status: "delivered", reason: "ok" };
    }
    if (resp.status === 404 || resp.status === 413) {
      return { status: "permanent_reject", reason: `http_${resp.status}` };
    }
    if (resp.status === 400 || resp.status === 422 || resp.status === 401 || resp.status === 403) {
      return { status: "dlq", reason: `http_${resp.status}` };
    }
    if (resp.status === 429 || resp.status >= 500) {
      return { status: "transient_error", reason: `http_${resp.status}` };
    }
    return { status: "dlq", reason: `unexpected_http_${resp.status}` };
  } catch (err) {
    const msg = err instanceof Error ? err.message : "network_error";
    return { status: "transient_error", reason: msg };
  }
}

async function enqueueForRetry(env, rawBuffer, envelopeTo, envelopeFrom, receivedAt, reason) {
  // Queues has a 128 KB message size limit; store larger payloads in R2 and pass the object key.
  if (env.BARAMAIL_RETRY_QUEUE) {
    if (rawBuffer.byteLength <= QUEUE_INLINE_LIMIT_BYTES) {
      await env.BARAMAIL_RETRY_QUEUE.send({
        rawBase64: arrayBufferToBase64(rawBuffer),
        envelopeTo,
        envelopeFrom,
        receivedAt,
        reason,
      });
      return true;
    }
    if (env.BARAMAIL_DLQ_BUCKET) {
      const key = `retry/${Date.now()}-${crypto.randomUUID()}.eml`;
      await env.BARAMAIL_DLQ_BUCKET.put(key, rawBuffer, {
        customMetadata: { envelopeTo, envelopeFrom, receivedAt, reason },
      });
      await env.BARAMAIL_RETRY_QUEUE.send({
        r2Key: key,
        envelopeTo,
        envelopeFrom,
        receivedAt,
        reason,
      });
      return true;
    }
  }

  if (env.BARAMAIL_DLQ_BUCKET) {
    await persistToDLQ(env, rawBuffer, envelopeTo, envelopeFrom, receivedAt, reason);
    return true;
  }

  return false;
}

async function persistToDLQ(env, rawBuffer, envelopeTo, envelopeFrom, receivedAt, reason) {
  console.error(JSON.stringify({
    event: "email_moved_to_dlq",
    to: envelopeTo,
    reason,
    size: rawBuffer.byteLength,
  }));

  if (env.BARAMAIL_DLQ_BUCKET) {
    const key = `dlq/${Date.now()}-${crypto.randomUUID()}.eml`;
    await env.BARAMAIL_DLQ_BUCKET.put(key, rawBuffer, {
      customMetadata: { envelopeTo, envelopeFrom, receivedAt, reason },
    });
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function base64ToArrayBuffer(b64) {
  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

