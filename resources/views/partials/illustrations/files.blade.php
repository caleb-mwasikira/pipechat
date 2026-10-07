{{-- A file travelling from one terminal to another, with its progress bar and hash. --}}
<svg viewBox="0 0 480 300" role="img" aria-label="A file being sent from one terminal to another" class="h-auto w-full">
    <rect x="1" y="1" width="478" height="298" class="fill-cyber-900 stroke-cyber-700" stroke-width="2"/>
    {{-- sender --}}
    <rect x="22" y="40" width="150" height="110" class="fill-cyber-800 stroke-cyber-600" stroke-width="2"/>
    <rect x="22" y="40" width="150" height="18" class="fill-cyber-700"/>
    <circle cx="35" cy="49" r="3" fill="#f87171"/><circle cx="46" cy="49" r="3" fill="#fcd34d"/><circle cx="57" cy="49" r="3" fill="#00ff66"/>
    <text x="32" y="80" class="fill-cyber-neon font-mono" font-size="10">$</text><text x="42" y="80" class="fill-gray-200 font-mono" font-size="10">/upload</text>
    <text x="32" y="96" class="fill-cyber-cyan font-mono" font-size="10">holiday.zip</text>
    <text x="32" y="124" class="fill-gray-500 font-mono" font-size="9">md5 9e107d9d</text>
    {{-- receiver --}}
    <rect x="308" y="150" width="150" height="110" class="fill-cyber-800 stroke-cyber-600" stroke-width="2"/>
    <rect x="308" y="150" width="150" height="18" class="fill-cyber-700"/>
    <circle cx="321" cy="159" r="3" fill="#f87171"/><circle cx="332" cy="159" r="3" fill="#fcd34d"/><circle cx="343" cy="159" r="3" fill="#00ff66"/>
    <text x="318" y="190" class="fill-cyber-neon font-mono" font-size="10">$</text><text x="328" y="190" class="fill-gray-200 font-mono" font-size="10">/download</text>
    <text x="318" y="206" class="fill-cyber-cyan font-mono" font-size="10">holiday.zip</text>
    <text x="318" y="234" class="fill-cyber-neon font-mono" font-size="9">✓ md5 matches</text>
    {{-- the path and the file in flight --}}
    <path d="M172 100 C 250 100, 230 205, 308 205" fill="none" class="stroke-cyber-600 illus-flow" stroke-width="2.5" stroke-dasharray="7 7"/>
    <g transform="translate(222 120)" class="illus-hop">
        <path d="M0 0 h26 l12 12 v34 h-38 z" class="fill-cyber-800 stroke-cyber-neon" stroke-width="2.5"/>
        <path d="M26 0 v12 h12" fill="none" class="stroke-cyber-neon" stroke-width="2.5"/>
        <text x="19" y="36" text-anchor="middle" class="fill-cyber-neon font-mono" font-size="9" font-weight="bold">ZIP</text>
    </g>
    {{-- progress bar --}}
    <text x="22" y="200" class="fill-gray-400 font-mono" font-size="10">⇡ uploading holiday.zip</text>
    <rect x="22" y="210" width="190" height="9" class="fill-cyber-700"/><rect x="22" y="210" width="124" height="9" class="fill-cyber-neon"/>
    <text x="22" y="238" class="fill-gray-300 font-mono" font-size="10">65%  7.8 MB / 12 MB  2.4 MB/s</text>
    <g transform="translate(24 252)"><rect width="22" height="16" rx="2" class="fill-none stroke-cyber-neon" stroke-width="1.8"/><path d="M5 6 V4 a6 6 0 0 1 12 0 V6" transform="translate(0 -4)" class="fill-none stroke-cyber-neon" stroke-width="1.8"/></g>
    <text x="54" y="265" class="fill-gray-500 font-mono" font-size="9">encrypted before it leaves</text>
</svg>
