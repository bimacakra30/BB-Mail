<template>
  <RouterView />
  <div v-if="toast.message.value" class="toast" role="status" aria-live="polite">{{ toast.message.value }}</div>
</template>

<script setup lang="ts">
import { useToast } from "./stores/toast";

const toast = useToast();
</script>
