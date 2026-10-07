@props(['icon' => 'fa-solid fa-lock', 'iconClass' => 'text-gray-400'])

<div class="relative">
    <i class="{{ $icon }} absolute left-3.5 top-3 {{ $iconClass }} text-xs"></i>
    <input type="password" {{ $attributes->except('type')->merge(['class' => 'w-full bg-cyber-900 border border-cyber-600 rounded-lg py-2.5 pl-10 pr-11 text-sm text-white focus:outline-none focus:border-cyber-neon transition-all']) }}>
    <button type="button" aria-label="Show password"
            onclick="const i = this.previousElementSibling; const show = i.type === 'password'; i.type = show ? 'text' : 'password'; this.querySelector('i').className = show ? 'fa-solid fa-eye-slash' : 'fa-solid fa-eye'"
            class="absolute right-3 top-2.5 text-gray-400 hover:text-cyber-neon focus:text-cyber-neon focus:outline-none transition-colors">
        <i class="fa-solid fa-eye"></i>
    </button>
</div>
