@extends('layouts.app')
@section('title', 'Admin')
@section('container', 'max-w-6xl')
@section('content')
@php $max = max(1, $signups->max('count')); @endphp
<div class="mb-8">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// admin</p>
    <h1 class="font-mono text-3xl font-black text-white">Dashboard</h1>
    <a href="{{ route('admin.binaries') }}" class="font-mono text-xs text-cyber-neon hover:underline"><i class="fa-solid fa-box-archive mr-1"></i>Manage binaries</a>
</div>

<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
    @foreach ($stats as $label => $value)
        <div class="corners border border-cyber-600 bg-cyber-800/90 p-5">
            <p class="font-mono text-[11px] uppercase tracking-wider text-gray-400">{{ $label }}</p>
            <p class="mt-2 font-mono text-3xl font-black text-cyber-neon text-glow-green">{{ number_format($value) }}</p>
        </div>
    @endforeach
</div>

<section class="mt-8 border border-cyber-600 bg-cyber-800/90 p-5">
    <h2 class="mb-4 font-mono text-xs font-bold uppercase tracking-wider text-white">Signups, last 30 days</h2>
    <div class="flex h-40 items-end gap-1" role="img" aria-label="Signups per day for the last 30 days">
        @foreach ($signups as $day)
            <div class="group relative flex-1 bg-cyber-neon/70 hover:bg-cyber-neon" style="height: {{ max(2, $day['count'] / $max * 100) }}%" title="{{ $day['date'] }}: {{ $day['count'] }}"></div>
        @endforeach
    </div>
    <div class="mt-2 flex justify-between font-mono text-[10px] text-gray-500"><span>{{ $signups->first()['date'] }}</span><span>{{ $signups->last()['date'] }}</span></div>
</section>

<section class="mt-8 overflow-x-auto border border-cyber-600 bg-cyber-800/90">
    <h2 class="border-b border-cyber-600 p-4 font-mono text-xs font-bold uppercase tracking-wider text-white">Newest users</h2>
    <table class="w-full font-mono text-xs">
        <thead><tr class="text-left text-gray-500"><th class="p-3">Name</th><th class="p-3">Email</th><th class="p-3">Joined</th></tr></thead>
        <tbody>
        @foreach ($recent as $user)
            <tr class="border-t border-cyber-700 text-gray-300">
                <td class="p-3">{{ $user->name }} @if ($user->is_admin)<span class="ml-1 text-cyber-purple">[admin]</span>@endif</td>
                <td class="p-3">{{ $user->email }}</td>
                <td class="p-3 text-gray-400">{{ $user->created_at->diffForHumans() }}</td>
            </tr>
        @endforeach
        </tbody>
    </table>
</section>
@endsection
