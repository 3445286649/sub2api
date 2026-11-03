<template>
  <!-- 8–12 petals falling near the tree (right side). Pure CSS transform/opacity — no canvas redraws. -->
  <div class="sfp-field" aria-hidden="true">
    <svg
      v-for="p in petals"
      :key="p.id"
      class="sfp-petal"
      viewBox="-10 -7 20 14"
      :style="p.style"
      focusable="false"
    >
      <path :d="p.path" />
    </svg>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { petalPathData } from './inkSakura/blossom'

// Deterministic "random" — avoids hydration mismatch and keeps animation stable across re-renders.
function rng(seed: number) {
  let s = seed
  return () => { s = (s * 1664525 + 1013904223) & 0xffffffff; return (s >>> 0) / 0xffffffff }
}

const PETAL_COUNT = 10

const petals = computed(() => {
  const r = rng(20261103)
  return Array.from({ length: PETAL_COUNT }, (_, i) => {
    const size   = 7 + r() * 5          // 7–12 px
    const x      = 55 + r() * 38        // 55–93 vw, near the tree on the right
    const delay  = r() * 14             // 0–14 s stagger
    const dur    = 9 + r() * 8          // 9–17 s fall cycle
    const drift  = (r() - 0.5) * 80    // horizontal swing ±40 px
    const rot    = r() * 360            // initial rotation
    const rotEnd = rot + (r() - 0.5) * 480

    return {
      id: i,
      path: petalPathData(10, 7, 1.5),
      style: {
        '--sfp-x':       `${x}vw`,
        '--sfp-drift':   `${drift}px`,
        '--sfp-rot':     `${rot}deg`,
        '--sfp-rot-end': `${rotEnd}deg`,
        '--sfp-size':    `${size}px`,
        '--sfp-dur':     `${dur}s`,
        '--sfp-delay':   `${delay}s`,
      } as Record<string, string>,
    }
  })
})
</script>

<style scoped>
.sfp-field {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 0;
  overflow: hidden;
}

.sfp-petal {
  position: absolute;
  top: -20px;
  left: var(--sfp-x);
  width: var(--sfp-size);
  height: var(--sfp-size);
  fill: rgba(234, 168, 190, 0.72);
  filter: drop-shadow(0 1px 2px rgba(200, 100, 140, 0.18));
  animation: sfp-fall var(--sfp-dur) var(--sfp-delay) infinite linear;
  transform-origin: 50% 50%;
  will-change: transform, opacity;
}

.dark .sfp-petal {
  fill: rgba(192, 138, 162, 0.55);
  filter: drop-shadow(0 1px 3px rgba(255, 180, 210, 0.22));
}

@keyframes sfp-fall {
  0%   { transform: translateY(-20px) translateX(0)                  rotate(var(--sfp-rot));     opacity: 0; }
  6%   { opacity: 1; }
  90%  { opacity: 0.85; }
  100% { transform: translateY(105vh) translateX(var(--sfp-drift))   rotate(var(--sfp-rot-end)); opacity: 0; }
}

/* Hide on narrow screens and when user prefers reduced motion. */
@media (max-width: 639px) { .sfp-field { display: none; } }
@media (prefers-reduced-motion: reduce) { .sfp-field { display: none; } }
@media print { .sfp-field { display: none; } }
</style>
