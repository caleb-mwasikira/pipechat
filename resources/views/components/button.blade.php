@props(['variant' => 'primary'])

@php
    $variants = [
        'primary' => 'bg-cyber-neon text-black hover:bg-emerald-400 neon-glow-green',
        'secondary' => 'bg-cyber-700 text-gray-300 hover:text-white',
    ];
@endphp

<button {{ $attributes->merge(['class' => 'w-full py-3 rounded-lg font-bold text-xs uppercase tracking-wider transition-all '.$variants[$variant]]) }}>
    {{ $slot }}
</button>
