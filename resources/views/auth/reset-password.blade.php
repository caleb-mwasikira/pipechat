@extends('layouts.app')
@section('title', 'Reset password')
@section('content')
<x-card icon="fa-lock">
    <form method="POST" action="{{ route('password.update') }}" class="space-y-4">
        @csrf
        <input type="hidden" name="token" value="{{ $token }}">
        <x-heading title="CHOOSE A NEW PASSWORD" />
        <div>
            <x-label for="email">Email</x-label>
            <x-input icon="fa-solid fa-envelope" type="email" name="email" id="email" value="{{ old('email', request('email')) }}" required />
            <x-input-error :messages="$errors->get('email')" />
        </div>
        <div>
            <x-label for="password">New Password</x-label>
            <x-password-input icon="fa-solid fa-key" name="password" id="password" required autofocus />
            <x-input-error :messages="$errors->get('password')" />
        </div>
        <div>
            <x-label for="password_confirmation">Confirm Password</x-label>
            <x-password-input icon="fa-solid fa-key" name="password_confirmation" id="password_confirmation" required />
        </div>
        <x-button type="submit"><i class="fa-solid fa-floppy-disk mr-2"></i>Update Password</x-button>
    </form>
</x-card>
@endsection
