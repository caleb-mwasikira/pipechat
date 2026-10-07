@props(['for' => null])

<label @if ($for) for="{{ $for }}" @endif {{ $attributes->merge(['class' => 'block text-xs font-bold uppercase tracking-wider text-gray-300 mb-1.5']) }}>
    {{ $slot }}
</label>
