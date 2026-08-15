<!--
  File:        webui/src/components/AchievementSprite.vue
  Project:     https://git.obth.eu/atjontv/kosync
  Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
-->
<!--
  The achievement cats, as one hidden sprite mounted once.

  They are drawn here rather than uploaded as files: an icon that ships with the
  code is versioned with it, recoloured by CSS, and costs no request. The whole
  set is eight cats in under 18 KB of markup.

  Every composition fits inside a circle of radius 46 about (60,60) and the disc
  is 50, so nothing touches the ring. The fur is two custom properties, which
  inherit into the shadow tree a <use> creates, so one drawing serves every coat.
-->
<template>
  <svg width="0" height="0" class="absolute" aria-hidden="true" focusable="false">
    <defs>
      <pattern id="halftone" width="6" height="6" patternUnits="userSpaceOnUse">
        <circle cx="3" cy="3" r="1.15" fill="rgba(20,18,14,0.10)" />
      </pattern>

      <clipPath id="disc"><circle cx="60" cy="60" r="50" /></clipPath>

      <g id="badge-frame">
        <circle
          cx="60"
          cy="60"
          r="55.5"
          fill="var(--ring, #c4763f)"
          stroke="#14120e"
          stroke-width="5"
        />
        <circle cx="60" cy="60" r="50" fill="#fff6e2" stroke="#14120e" stroke-width="4" />
      </g>
      <g id="badge-grain">
        <circle cx="60" cy="60" r="50" fill="url(#halftone)" />
        <circle cx="60" cy="60" r="50" fill="none" stroke="#14120e" stroke-width="4" />
      </g>

      <!-- the head every cat shares: centre (60,54), rx 29, ry 25 -->
      <g id="cat-head">
        <path
          d="M35 43 L32 24 L56 33 Z"
          fill="var(--fur)"
          stroke="#14120e"
          stroke-width="4"
          stroke-linejoin="round"
        />
        <path
          d="M85 43 L88 24 L64 33 Z"
          fill="var(--fur)"
          stroke="#14120e"
          stroke-width="4"
          stroke-linejoin="round"
        />
        <path d="M38 40 L36 28 L50 34 Z" fill="#f58fa8" />
        <path d="M82 40 L84 28 L70 34 Z" fill="#f58fa8" />
        <ellipse
          cx="60"
          cy="54"
          rx="29"
          ry="25"
          fill="var(--fur)"
          stroke="#14120e"
          stroke-width="4"
        />
        <path
          d="M53 32 l2.5 8 M60 30 v9 M67 32 l-2.5 8"
          stroke="var(--fur-shade)"
          stroke-width="4.5"
          stroke-linecap="round"
          fill="none"
        />
        <path
          d="M41 62 L24 58 M41 67 L23 69 M79 62 L96 58 M79 67 L97 69"
          stroke="#14120e"
          stroke-width="3"
          stroke-linecap="round"
          fill="none"
        />
        <ellipse
          cx="49"
          cy="51"
          rx="8.5"
          ry="9.5"
          fill="#fffdf5"
          stroke="#14120e"
          stroke-width="3.4"
        />
        <ellipse
          cx="71"
          cy="51"
          rx="8.5"
          ry="9.5"
          fill="#fffdf5"
          stroke="#14120e"
          stroke-width="3.4"
        />
        <ellipse cx="50.3" cy="52.4" rx="4.2" ry="5.6" fill="#14120e" />
        <ellipse cx="72.3" cy="52.4" rx="4.2" ry="5.6" fill="#14120e" />
        <circle cx="47.9" cy="48.6" r="2.1" fill="#fffdf5" />
        <circle cx="69.9" cy="48.6" r="2.1" fill="#fffdf5" />
        <path
          d="M54.5 60 H65.5 L60 66 Z"
          fill="#f58fa8"
          stroke="#14120e"
          stroke-width="2.9"
          stroke-linejoin="round"
        />
        <path
          d="M60 66 q-5.5 6.5 -11 1.4 M60 66 q5.5 6.5 11 1.4"
          fill="none"
          stroke="#14120e"
          stroke-width="3.4"
          stroke-linecap="round"
        />
      </g>

      <!-- 1. cat behind an open book -->
      <symbol id="ach-first" viewBox="0 0 120 120">
        <use href="#badge-frame" />
        <g clip-path="url(#disc)">
          <use href="#cat-head" />
          <path
            d="M60 87 C49 81 38 81 30 83 L30 93 C40 91 51 91 60 98 C69 91 80 91 90 93 L90 83
                   C82 81 71 81 60 87 Z"
            fill="#d9342a"
            stroke="#14120e"
            stroke-width="4"
            stroke-linejoin="round"
          />
          <path
            d="M60 84 C50 78 40 78 33 80 L33 89 C42 87 52 87 60 94 Z"
            fill="#fff6e2"
            stroke="#14120e"
            stroke-width="3.6"
            stroke-linejoin="round"
          />
          <path
            d="M60 84 C70 78 80 78 87 80 L87 89 C78 87 68 87 60 94 Z"
            fill="#fff6e2"
            stroke="#14120e"
            stroke-width="3.6"
            stroke-linejoin="round"
          />
          <path
            d="M38 83 L54 85 M38 88 L54 90 M66 85 L82 83 M66 90 L82 88"
            stroke="#14120e"
            stroke-width="2.2"
            stroke-linecap="round"
            opacity=".4"
          />
          <rect
            x="28"
            y="78"
            width="19"
            height="12"
            rx="6"
            fill="var(--fur)"
            stroke="#14120e"
            stroke-width="3.8"
          />
          <rect
            x="73"
            y="78"
            width="19"
            height="12"
            rx="6"
            fill="var(--fur)"
            stroke="#14120e"
            stroke-width="3.8"
          />
          <path
            d="M34 78.5 v5.5 M40.5 78.5 v5.5 M79 78.5 v5.5 M85.5 78.5 v5.5"
            stroke="#14120e"
            stroke-width="2.2"
            stroke-linecap="round"
            opacity=".5"
          />
        </g>
        <use href="#badge-grain" />
      </symbol>

      <!-- 2. cat batting a loose page -->
      <symbol id="ach-page" viewBox="0 0 120 120">
        <use href="#badge-frame" />
        <g clip-path="url(#disc)">
          <path
            d="M72 30 L94 36 L88 60 L66 54 Z"
            fill="#fffdf5"
            stroke="#14120e"
            stroke-width="4"
            stroke-linejoin="round"
          />
          <path
            d="M74 39 L88 42.5 M73 46 L87 49.5 M72 53 L82 55.5"
            stroke="#14120e"
            stroke-width="2.2"
            stroke-linecap="round"
            opacity=".4"
          />
          <path
            d="M36 98 q-1 -16 11 -22 q12 -6 25 -2"
            fill="none"
            stroke="#14120e"
            stroke-width="11"
            stroke-linecap="round"
          />
          <path
            d="M36 98 q-1 -16 11 -22 q12 -6 25 -2"
            fill="none"
            stroke="var(--fur)"
            stroke-width="5"
            stroke-linecap="round"
          />
          <g transform="translate(54,58) scale(0.9) translate(-60,-54)">
            <use href="#cat-head" />
          </g>
          <g transform="rotate(-26 76 70)">
            <rect
              x="66"
              y="64"
              width="20"
              height="12"
              rx="6"
              fill="var(--fur)"
              stroke="#14120e"
              stroke-width="3.8"
            />
            <path
              d="M72 64.5 v5.5 M79 64.5 v5.5"
              stroke="#14120e"
              stroke-width="2.2"
              stroke-linecap="round"
              opacity=".5"
            />
          </g>
        </g>
        <use href="#badge-grain" />
      </symbol>

      <!-- 3. cat perched on a stack -->
      <symbol id="ach-shelf" viewBox="0 0 120 120">
        <use href="#badge-frame" />
        <g clip-path="url(#disc)">
          <g transform="translate(60,44) scale(0.74) translate(-60,-54)">
            <use href="#cat-head" />
          </g>
          <rect
            x="34"
            y="86"
            width="52"
            height="11"
            rx="2"
            fill="#2c5fd0"
            stroke="#14120e"
            stroke-width="3.8"
          />
          <rect
            x="38"
            y="76"
            width="44"
            height="11"
            rx="2"
            fill="#f0b800"
            stroke="#14120e"
            stroke-width="3.8"
          />
          <rect
            x="42"
            y="66"
            width="36"
            height="11"
            rx="2"
            fill="#d9342a"
            stroke="#14120e"
            stroke-width="3.8"
          />
          <path
            d="M38 92 h44 M42 82 h36 M46 72 h28"
            stroke="#fff6e2"
            stroke-width="2.4"
            opacity=".75"
          />
          <rect
            x="46"
            y="62"
            width="13"
            height="9"
            rx="4.5"
            fill="var(--fur)"
            stroke="#14120e"
            stroke-width="3.6"
          />
          <rect
            x="62"
            y="62"
            width="13"
            height="9"
            rx="4.5"
            fill="var(--fur)"
            stroke="#14120e"
            stroke-width="3.6"
          />
        </g>
        <use href="#badge-grain" />
      </symbol>

      <!-- 4. cat lit by an e-reader at night -->
      <symbol id="ach-night" viewBox="0 0 120 120">
        <use href="#badge-frame" />
        <g clip-path="url(#disc)">
          <circle cx="60" cy="60" r="50" fill="#2b3550" />
          <path
            d="M95 32 a11 11 0 1 1 -10.5 -10.9 a9 9 0 0 0 10.5 10.9 Z"
            fill="#f0b800"
            stroke="#14120e"
            stroke-width="3.2"
            stroke-linejoin="round"
          />
          <path
            d="M30 24 l2.2 5.3 5.3 2.2 -5.3 2.2 -2.2 5.3 -2.2 -5.3 -5.3 -2.2 5.3 -2.2 Z"
            fill="#fffdf5"
          />
          <path
            d="M43 44 l1.5 3.6 3.6 1.5 -3.6 1.5 -1.5 3.6 -1.5 -3.6 -3.6 -1.5 3.6 -1.5 Z"
            fill="#fffdf5"
            opacity=".8"
          />
          <g transform="translate(60,48) scale(0.86) translate(-60,-54)">
            <use href="#cat-head" />
          </g>
          <path d="M42 80 L26 46 L94 46 L78 80 Z" fill="#bff3d8" opacity=".2" />
          <rect
            x="40"
            y="76"
            width="40"
            height="22"
            rx="4"
            fill="#4a5c6a"
            stroke="#14120e"
            stroke-width="3.8"
          />
          <rect
            x="45"
            y="80"
            width="30"
            height="14"
            rx="1.5"
            fill="#bff3d8"
            stroke="#14120e"
            stroke-width="2.4"
          />
          <path
            d="M49 84 h22 M49 88 h22 M49 91.5 h13"
            stroke="#14120e"
            stroke-width="2"
            stroke-linecap="round"
            opacity=".4"
          />
        </g>
        <use href="#badge-grain" />
      </symbol>

      <!-- 5. curled sleeping cat -->
      <symbol id="ach-streak" viewBox="0 0 120 120">
        <use href="#badge-frame" />
        <g clip-path="url(#disc)">
          <path
            d="M84 66 q17 8 8 21 q-11 15 -31 9"
            fill="none"
            stroke="#14120e"
            stroke-width="12"
            stroke-linecap="round"
          />
          <path
            d="M84 66 q17 8 8 21 q-11 15 -31 9"
            fill="none"
            stroke="var(--fur)"
            stroke-width="5.5"
            stroke-linecap="round"
          />
          <ellipse
            cx="60"
            cy="72"
            rx="34"
            ry="23"
            fill="var(--fur)"
            stroke="#14120e"
            stroke-width="4"
          />
          <path
            d="M32 82 q28 12 56 0"
            fill="none"
            stroke="var(--fur-shade)"
            stroke-width="4"
            stroke-linecap="round"
            opacity=".8"
          />
          <path
            d="M27 57 L22 43 L38 51 Z"
            fill="var(--fur)"
            stroke="#14120e"
            stroke-width="4"
            stroke-linejoin="round"
          />
          <path
            d="M43 55 L49 41 L55 56 Z"
            fill="var(--fur)"
            stroke="#14120e"
            stroke-width="4"
            stroke-linejoin="round"
          />
          <circle cx="38" cy="69" r="18" fill="var(--fur)" stroke="#14120e" stroke-width="4" />
          <path
            d="M27 67 q4.5 5 9 0 M41 67 q4.5 5 9 0"
            fill="none"
            stroke="#14120e"
            stroke-width="3.4"
            stroke-linecap="round"
          />
          <path
            d="M33.5 76 H42.5 L38 81 Z"
            fill="#f58fa8"
            stroke="#14120e"
            stroke-width="2.6"
            stroke-linejoin="round"
          />
          <g class="display" fill="#14120e">
            <text x="64" y="42" font-size="13" transform="rotate(-8 64 42)">z</text>
            <text x="72" y="32" font-size="16" transform="rotate(-8 72 32)">Z</text>
            <text x="81" y="21" font-size="19" transform="rotate(-8 81 21)">Z</text>
          </g>
        </g>
        <use href="#badge-grain" />
      </symbol>

      <!-- 6. cat on the windowsill with the sun coming up -->
      <symbol id="ach-dawn" viewBox="0 0 120 120">
        <use href="#badge-frame" />
        <g clip-path="url(#disc)">
          <circle cx="60" cy="60" r="50" fill="#ffe9bd" />
          <g fill="#f0b800" opacity=".85">
            <path
              d="M60 74 L122 74 L121.6 80.7 Z M60 74 L117.3 97.7 L114.4 103.8 Z
                 M60 74 L103.8 117.8 L98.9 122.3 Z M60 74 L16.2 117.8 L11.7 112.9 Z
                 M60 74 L2.7 97.7 L0.5 91.4 Z M60 74 L-2 74 L-1.6 67.3 Z
                 M60 74 L2.7 50.3 L5.6 44.2 Z M60 74 L16.2 30.2 L21.1 25.7 Z
                 M60 74 L36.3 16.7 L42.6 14.5 Z M60 74 L60 12 L66.7 12.4 Z
                 M60 74 L83.7 16.7 L89.8 19.6 Z M60 74 L103.8 30.2 L108.3 35.1 Z
                 M60 74 L117.3 50.3 L119.5 56.6 Z"
            />
          </g>
          <g transform="translate(60,50) scale(0.8) translate(-60,-54)">
            <use href="#cat-head" />
          </g>
          <g transform="rotate(-5 60 84)">
            <path
              d="M60 81 C49 74 38 74 30 76 L30 88 C40 86 51 86 60 93 C69 86 80 86 90 88 L90 76
                 C82 74 71 74 60 81 Z"
              fill="#2fa36b"
              stroke="#14120e"
              stroke-width="3.4"
              stroke-linejoin="round"
            />
            <path
              d="M60 80 C51 75 42 75 36 76.5 L36 83 C44 81.5 53 82 60 87 Z"
              fill="#fffdf5"
              stroke="#14120e"
              stroke-width="2.6"
              stroke-linejoin="round"
            />
            <path
              d="M60 80 C69 75 78 75 84 76.5 L84 83 C76 81.5 67 82 60 87 Z"
              fill="#fffdf5"
              stroke="#14120e"
              stroke-width="2.6"
              stroke-linejoin="round"
            />
            <path
              d="M40 79 L54 81 M40 83 L54 85 M66 81 L80 79 M66 85 L80 83"
              stroke="#14120e"
              stroke-width="1.8"
              stroke-linecap="round"
              opacity=".4"
            />
          </g>
        </g>
        <use href="#badge-grain" />
      </symbol>

      <!-- 7. cat behind the pile of pages one day made -->
      <symbol id="ach-day" viewBox="0 0 120 120">
        <use href="#badge-frame" />
        <g clip-path="url(#disc)">
          <g transform="translate(60,46) scale(0.78) translate(-60,-54)">
            <use href="#cat-head" />
          </g>
          <rect
            x="32"
            y="70"
            width="56"
            height="24"
            rx="2"
            fill="#fffdf5"
            stroke="#14120e"
            stroke-width="4"
          />
          <path
            d="M36 75 H84 M36 79.5 H84 M36 84 H84 M36 88.5 H84"
            stroke="#14120e"
            stroke-width="2.2"
            stroke-linecap="round"
            opacity=".35"
          />
          <path
            d="M64 70 h11 v27 l-5.5 -5 -5.5 5 Z"
            fill="#d9342a"
            stroke="#14120e"
            stroke-width="3.4"
            stroke-linejoin="round"
          />
        </g>
        <use href="#badge-grain" />
      </symbol>

      <!-- 8. cat inside the arrow of a book gone round again -->
      <symbol id="ach-again" viewBox="0 0 120 120">
        <use href="#badge-frame" />
        <g clip-path="url(#disc)">
          <path
            d="M28.8 78 A36 36 0 1 1 80.6 89.5"
            fill="none"
            stroke="#14120e"
            stroke-width="11"
            stroke-linecap="round"
          />
          <path
            d="M28.8 78 A36 36 0 1 1 80.6 89.5"
            fill="none"
            stroke="#2c5fd0"
            stroke-width="5"
            stroke-linecap="round"
          />
          <path
            d="M70 96.9 L83.9 97.6 L74.1 83.7 Z"
            fill="#2c5fd0"
            stroke="#14120e"
            stroke-width="3.2"
            stroke-linejoin="round"
          />
          <g transform="translate(58,52) scale(0.55) translate(-60,-54)">
            <use href="#cat-head" />
          </g>
          <rect
            x="42"
            y="67"
            width="34"
            height="14"
            rx="2"
            fill="#d9342a"
            stroke="#14120e"
            stroke-width="3.8"
          />
          <path d="M46 74 h26" stroke="#fff6e2" stroke-width="2.6" opacity=".75" />
        </g>
        <use href="#badge-grain" />
      </symbol>
    </defs>
  </svg>
</template>
