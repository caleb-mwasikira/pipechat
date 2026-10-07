@props(['title', 'sub' => null])

<div class="text-center pb-2">
    <h2 class="text-lg font-bold text-white tracking-wider">{{ $title }}</h2>
    @if ($sub)<p class="text-[11px] text-gray-400">{{ $sub }}</p>@endif
</div>
