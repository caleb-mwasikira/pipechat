@props(['icon' => null, 'iconClass' => 'text-gray-400'])

<div class="relative">
    @if ($icon)
        <i class="{{ $icon }} absolute left-3.5 top-3 {{ $iconClass }} text-xs"></i>
    @endif

    <input {{ $attributes->merge(['class' => 'w-full bg-cyber-900 border border-cyber-600 rounded-lg py-2.5 text-sm text-white focus:outline-none focus:border-cyber-neon transition-all '.($icon ? 'pl-10 pr-4' : 'px-4')]) }}>
</div>
