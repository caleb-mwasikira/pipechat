@props(['icon' => 'fa-key'])

<div class="max-w-md mx-auto bg-cyber-800/90 border border-cyber-600 rounded-xl p-4 sm:p-6 md:p-8 shadow-2xl relative overflow-hidden">
    <div class="absolute -right-10 -bottom-10 opacity-5 pointer-events-none">
        <i class="fa-solid {{ $icon }} text-9xl text-cyber-neon"></i>
    </div>
    <div class="relative z-10">{{ $slot }}</div>
</div>
