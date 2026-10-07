@extends('layouts.app')
@section('title', 'Forgot password')
@section('content')
<x-card icon="fa-envelope">
    <x-status />
    <form method="POST" action="{{ route('password.email') }}" class="space-y-4">
        @csrf
        <x-heading title="RESET PASSWORD" sub="We'll email you a link to choose a new one." />
        <div>
            <x-label for="email">Email</x-label>
            <x-input icon="fa-solid fa-envelope" type="email" name="email" id="email" value="{{ old('email') }}" required autofocus />
            <x-input-error :messages="$errors->get('email')" />
        </div>
        <x-button type="submit"><i class="fa-solid fa-paper-plane mr-2"></i>Send Reset Link</x-button>
        <div class="text-center"><a href="{{ route('login') }}" class="text-[11px] text-gray-400 hover:text-cyber-neon transition-colors">Back to login.</a></div>
    </form>
</x-card>
@endsection
