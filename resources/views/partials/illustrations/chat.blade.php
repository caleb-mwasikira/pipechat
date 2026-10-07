{{-- Two terminals talking to each other, with a lock between them: end-to-end encrypted chat. --}}
<svg viewBox="0 0 480 300" role="img" aria-label="Two terminal windows exchanging encrypted messages" class="h-auto w-full">
    <defs>
        <linearGradient id="chat-glow" x1="0" x2="1"><stop offset="0" stop-color="#00ff66" stop-opacity=".0"/><stop offset=".5" stop-color="#00ff66" stop-opacity=".55"/><stop offset="1" stop-color="#00f0ff" stop-opacity="0"/></linearGradient>
    </defs>
    <rect x="1" y="1" width="478" height="298" class="fill-cyber-900 stroke-cyber-700" stroke-width="2"/>
    {{-- left window --}}
    <g>
        <rect x="24" y="38" width="190" height="130" class="fill-cyber-800 stroke-cyber-600" stroke-width="2"/>
        <rect x="24" y="38" width="190" height="20" class="fill-cyber-700"/>
        <circle cx="38" cy="48" r="3.5" fill="#f87171"/><circle cx="50" cy="48" r="3.5" fill="#fcd34d"/><circle cx="62" cy="48" r="3.5" fill="#00ff66"/>
        <text x="76" y="52" class="fill-gray-500 font-mono" font-size="9">alice</text>
        <text x="36" y="80" class="fill-cyber-cyan font-mono" font-size="11">alice</text><text x="76" y="80" class="fill-gray-300 font-mono" font-size="11">&gt; deploy at 5?</text>
        <text x="36" y="104" class="fill-cyber-purple font-mono" font-size="11">bob</text><text x="64" y="104" class="fill-gray-300 font-mono" font-size="11">&gt; always. ship it.</text>
        <text x="36" y="128" class="fill-cyber-cyan font-mono" font-size="11">alice</text><text x="76" y="128" class="fill-gray-300 font-mono" font-size="11">&gt; 🚀</text>
        <text x="36" y="154" class="fill-cyber-neon font-mono" font-size="11">&gt; _</text>
    </g>
    {{-- right window --}}
    <g>
        <rect x="266" y="132" width="190" height="130" class="fill-cyber-800 stroke-cyber-600" stroke-width="2"/>
        <rect x="266" y="132" width="190" height="20" class="fill-cyber-700"/>
        <circle cx="280" cy="142" r="3.5" fill="#f87171"/><circle cx="292" cy="142" r="3.5" fill="#fcd34d"/><circle cx="304" cy="142" r="3.5" fill="#00ff66"/>
        <text x="318" y="146" class="fill-gray-500 font-mono" font-size="9">bob</text>
        <text x="278" y="174" class="fill-cyber-cyan font-mono" font-size="11">alice</text><text x="318" y="174" class="fill-gray-300 font-mono" font-size="11">&gt; deploy at 5?</text>
        <text x="278" y="198" class="fill-cyber-purple font-mono" font-size="11">bob</text><text x="306" y="198" class="fill-gray-300 font-mono" font-size="11">&gt; always. ship it.</text>
        <text x="278" y="222" class="fill-cyber-cyan font-mono" font-size="11">alice</text><text x="318" y="222" class="fill-gray-300 font-mono" font-size="11">&gt; 🚀</text>
        <text x="278" y="248" class="fill-cyber-neon font-mono" font-size="11">&gt; _</text>
    </g>
    {{-- the encrypted path between them --}}
    <path d="M214 110 C 250 110, 240 200, 266 200" fill="none" stroke="url(#chat-glow)" stroke-width="3" stroke-dasharray="6 6" class="illus-flow"/>
    <g transform="translate(222 134)">
        <rect x="0" y="12" width="36" height="28" class="fill-cyber-900 stroke-cyber-neon" stroke-width="2"/>
        <path d="M8 12 V6 a10 10 0 0 1 20 0 V12" fill="none" class="stroke-cyber-neon" stroke-width="2.5"/>
        <circle cx="18" cy="26" r="3.5" class="fill-cyber-neon"/><rect x="16.5" y="27" width="3" height="8" class="fill-cyber-neon"/>
    </g>
    <text x="240" y="288" text-anchor="middle" class="fill-gray-500 font-mono" font-size="10">only you and your group can read it</text>
</svg>
