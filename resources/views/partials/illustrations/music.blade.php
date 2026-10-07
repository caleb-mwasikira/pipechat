{{-- A terminal playing the group's library, with an equaliser. --}}
<svg viewBox="0 0 480 300" role="img" aria-label="A terminal playing songs from a shared music library" class="h-auto w-full">
    <rect x="1" y="1" width="478" height="298" class="fill-cyber-900 stroke-cyber-700" stroke-width="2"/>
    <rect x="30" y="30" width="250" height="240" class="fill-cyber-800 stroke-cyber-600" stroke-width="2"/>
    <rect x="30" y="30" width="250" height="20" class="fill-cyber-700"/>
    <circle cx="44" cy="40" r="3.5" fill="#f87171"/><circle cx="56" cy="40" r="3.5" fill="#fcd34d"/><circle cx="68" cy="40" r="3.5" fill="#00ff66"/>
    <text x="82" y="44" class="fill-gray-500 font-mono" font-size="9">music</text>
    <text x="44" y="76" class="fill-cyber-neon font-mono" font-size="11">$</text><text x="56" y="76" class="fill-gray-200 font-mono" font-size="11">/play</text>
    <text x="44" y="104" class="fill-cyber-neon font-mono" font-size="11">♪</text><text x="60" y="104" class="fill-white font-mono" font-size="11" font-weight="bold">Blue Hour</text><text x="236" y="104" text-anchor="end" class="fill-gray-500 font-mono" font-size="9">3:42</text>
    <text x="44" y="128" class="fill-gray-600 font-mono" font-size="11">·</text><text x="60" y="128" class="fill-gray-400 font-mono" font-size="11">Night Drive</text>
    <text x="44" y="152" class="fill-gray-600 font-mono" font-size="11">·</text><text x="60" y="152" class="fill-gray-400 font-mono" font-size="11">Second Song</text>
    <text x="44" y="176" class="fill-gray-600 font-mono" font-size="11">·</text><text x="60" y="176" class="fill-gray-400 font-mono" font-size="11">Static Bloom</text>
    {{-- progress --}}
    <rect x="44" y="214" width="222" height="4" class="fill-cyber-600"/><rect x="44" y="214" width="96" height="4" class="fill-cyber-neon"/><circle cx="140" cy="216" r="5" class="fill-cyber-neon"/>
    <text x="44" y="244" class="fill-gray-500 font-mono" font-size="9">/next · /shuffle · /repeat all</text>
    {{-- equaliser --}}
    <g class="illus-eq" transform="translate(318 250)">
        <rect x="0" y="-60" width="14" height="60" rx="2" class="fill-cyber-neon eq-bar" style="--s:.9;--d:0s"/>
        <rect x="24" y="-100" width="14" height="100" rx="2" class="fill-cyber-cyan eq-bar" style="--s:.7;--d:-.3s"/>
        <rect x="48" y="-140" width="14" height="140" rx="2" class="fill-cyber-neon eq-bar" style="--s:.8;--d:-.6s"/>
        <rect x="72" y="-90" width="14" height="90" rx="2" class="fill-cyber-purple eq-bar" style="--s:1;--d:-.15s"/>
        <rect x="96" y="-120" width="14" height="120" rx="2" class="fill-cyber-cyan eq-bar" style="--s:.75;--d:-.45s"/>
        <rect x="120" y="-70" width="14" height="70" rx="2" class="fill-cyber-neon eq-bar" style="--s:.95;--d:-.75s"/>
    </g>
    <path d="M330 56 v34 a9 9 0 1 1 -5 -8 V62 l30 -7 v28 a9 9 0 1 1 -5 -8 V52 z" class="fill-none stroke-cyber-neon" stroke-width="2.5" stroke-linejoin="round" opacity=".8"/>
</svg>
