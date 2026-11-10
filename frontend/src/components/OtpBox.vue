<template>
  <div
    class="otp"
    :class="{ 'is-copied': copied }"
    role="button"
    tabindex="0"
    :aria-label="copied ? `Kode ${code} tersalin` : `Salin kode ${code}`"
    @click.stop="copy"
    @keydown.enter.prevent.stop="copy"
    @keydown.space.prevent.stop="copy"
  >
    <span class="otp-label" aria-live="polite">{{ copied ? "Kode tersalin" : "Ketuk untuk salin kode" }}</span>
    <span class="otp-code">{{ code }}</span>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";

const props = defineProps<{ code: string }>();

const copied = ref<boolean>(false);
let timer: number | null = null;

async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.code);
  } catch {
    return;
  }
  copied.value = true;
  if (timer !== null) clearTimeout(timer);
  timer = window.setTimeout(() => {
    copied.value = false;
    timer = null;
  }, 1400);
}
</script>

