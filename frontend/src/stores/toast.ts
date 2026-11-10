import { ref } from "vue";

const message = ref<string>("");
let timer: number | null = null;

export function useToast() {
  function show(text: string, durationMs = 2200): void {
    message.value = text;
    if (timer !== null) clearTimeout(timer);
    timer = window.setTimeout(() => {
      message.value = "";
      timer = null;
    }, durationMs);
  }

  return { message, show };
}

