@extends('layouts.app')
@section('title', 'Invitation')
@section('content')
<x-card icon="fa-user-plus">
    <x-heading title="YOU'RE INVITED" sub="Join a group on Pipechat." />
    <div class="my-5 border border-cyber-600 bg-cyber-900/60 p-4 text-center">
        <p class="break-all font-mono text-xl font-black text-white">{{ $group->name }}</p>
        <p class="mt-1 font-mono text-xs text-gray-400">
            <i class="fa-solid {{ $group->is_public ? 'fa-globe' : 'fa-lock' }} mr-1"></i>{{ $group->is_public ? 'public' : 'private' }}
            &middot; made by {{ $owner ? '@'.$owner->handle : 'a removed user' }}
            &middot; {{ $members }} {{ \Illuminate\Support\Str::plural('member', $members) }}
        </p>
    </div>
    <form method="POST" action="{{ route('invite.join', $token) }}">
        @csrf
        <x-button type="submit"><i class="fa-solid fa-right-to-bracket mr-2"></i>Join {{ $group->name }}</x-button>
    </form>
    <p class="mt-4 text-center font-mono text-[11px] text-gray-500">Chats are end to end encrypted. After you join, the group appears in your terminal.</p>
</x-card>
@endsection
