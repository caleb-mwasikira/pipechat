@props(['messages'])

@if ($messages)
    <ul {{ $attributes->merge(['class' => 'mt-1.5 space-y-1']) }}>
        @foreach ((array) $messages as $message)
            <li class="text-xs text-cyber-danger">{{ $message }}</li>
        @endforeach
    </ul>
@endif
