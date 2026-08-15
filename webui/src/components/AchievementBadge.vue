<!--
  File:        webui/src/components/AchievementBadge.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    /** The sprite symbol to draw, such as "ach-first". */
    icon: string
    /** The coat: ginger, grey, cream, soot or calico. */
    fur: string
    /** 1, 2 or 3. Zero means not earned, which is drawn drained of colour. */
    tier?: number
    /** Accessible name. The tier is appended, so pass the badge's own name. */
    label?: string
  }>(),
  { tier: 0, label: '' },
)

// The ring is the tier. One drawing per rule and three metals is what keeps
// eight achievements at eight drawings instead of twenty-four.
const rings = ['bronze', 'silver', 'gold']

const classes = computed(() => [
  `fur-${props.fur}`,
  `tier-${rings[Math.max(props.tier, 1) - 1] ?? 'bronze'}`,
  { locked: props.tier < 1 },
])

const title = computed(() => {
  if (!props.label) return undefined
  if (props.tier < 1) return `${props.label} — not earned yet`

  return `${props.label} — ${rings[props.tier - 1]}`
})
</script>

<template>
  <svg
    class="badge"
    :class="classes"
    viewBox="0 0 120 120"
    role="img"
    :aria-label="title"
    :aria-hidden="title ? undefined : true"
  >
    <title v-if="title">{{ title }}</title>
    <use :href="`#${icon}`" />
  </svg>
</template>

<style scoped>
.badge {
  width: 100%;
  height: auto;
  display: block;
  overflow: visible;
}

/* The coats. A new one costs two hex values and no new drawing. */
.fur-ginger {
  --fur: #f3a03c;
  --fur-shade: #cf7622;
}
.fur-grey {
  --fur: #b6c0ca;
  --fur-shade: #8a96a3;
}
.fur-cream {
  --fur: #fadfb2;
  --fur-shade: #ddb377;
}
.fur-soot {
  --fur: #55504a;
  --fur-shade: #38342f;
}
.fur-calico {
  --fur: #f6ece0;
  --fur-shade: #d8c3a8;
}

.tier-bronze {
  --ring: #c4763f;
}
.tier-silver {
  --ring: #adb8c2;
}
.tier-gold {
  --ring: #f0b800;
}

/* Not earned is the same drawing drained of colour, so what is still to come is
   visible rather than absent. */
.locked {
  filter: grayscale(1);
  opacity: 0.32;
}
</style>
