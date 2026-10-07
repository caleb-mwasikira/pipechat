@props(['title' => 'terminal', 'accent' => 'neon'])

<div {{ $attributes->merge(['class' => 'border border-cyber-600 bg-cyber-800 shadow-2xl']) }}>
    <div class="flex items-center gap-2 border-b border-cyber-600 bg-cyber-700/60 px-3 py-2">
        <span class="h-2.5 w-2.5 bg-red-400/80"></span>
        <span class="h-2.5 w-2.5 bg-amber-300/80"></span>
        <span class="h-2.5 w-2.5 {{ $accent === 'cyan' ? 'bg-cyber-cyan' : 'bg-cyber-neon' }}"></span>
        <span class="ml-2 truncate font-mono text-[11px] text-gray-400">{{ $title }}</span>
    </div>
    <div class="p-4 font-mono text-xs leading-relaxed sm:p-5 sm:text-[13px]">
        {{ $slot }}
    </div>
</div>
