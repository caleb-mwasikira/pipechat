@extends('layouts.app')
@section('container', 'max-w-6xl')
@section('content')
<section class="relative py-10 sm:py-16">
    <div class="term-grid absolute inset-0 -z-10"></div>
    <div class="grid items-center gap-10 lg:grid-cols-2">
        <div class="space-y-6">
            <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// terminal chat</p>
            <h1 class="font-mono text-4xl font-black leading-tight text-white sm:text-5xl">Chat from your terminal<span class="caret"></span></h1>
            <p class="max-w-md text-sm leading-relaxed text-gray-400">Pipechat is an end-to-end encrypted chat client that lives in your shell. Create an account, grab the binary for Linux or Windows, and connect.</p>
            <div class="flex flex-wrap gap-3">
                <a href="#features" class="border border-cyber-600 px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon">See what it does</a>
                <a href="{{ route('download') }}" class="bg-cyber-neon px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-download mr-2"></i>Download</a>
                @guest<a href="{{ route('login') }}" class="border border-cyber-600 px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon">Login</a>@endguest
            </div>
        </div>
        <x-window title="pipechat" class="corners">
            <p class="term-line" style="--d:.1s"><span class="text-cyber-neon">$</span> ./pipechat-linux-amd64</p>
            <p class="term-line text-gray-400" style="--d:.5s">connected to chat server</p>
            <p class="term-line" style="--d:.9s"><span class="text-cyber-cyan">alice</span> &gt; anyone up for a deploy?</p>
            <p class="term-line" style="--d:1.3s"><span class="text-cyber-purple">bob</span> &gt; always. ship it.</p>
            <p class="term-line" style="--d:1.7s"><span class="text-cyber-neon">$</span><span class="caret"></span></p>
        </x-window>
    </div>
</section>

{{-- What it does: three things, each with a small picture. --}}
<section id="features" class="py-10 sm:py-16" aria-labelledby="features-title">
    <p class="font-mono text-xs uppercase tracking-[0.2em] text-cyber-neon">// features</p>
    <h2 id="features-title" class="mb-12 font-mono text-3xl font-black text-white sm:text-4xl">Everything stays in your terminal</h2>

    @php
        $features = [
            ['n' => '01', 'icon' => 'fa-terminal', 'title' => 'Chat from your terminal', 'illustration' => 'chat',
             'text' => 'Groups and private chats in a fast, keyboard-driven client for Linux and Windows. Messages are end-to-end encrypted, so only the people in a chat can read them, and what you missed while away is waiting when you log in.',
             'points' => ['Public and private groups, invites and invite links', 'Search every chat, copy text, scroll with the wheel', 'Updates itself when a new version is out']],
            ['n' => '02', 'icon' => 'fa-music', 'title' => 'Listen to group music from the terminal', 'illustration' => 'music',
             'text' => 'Everyone uploads songs to one shared library on the website. In the terminal, type /play and the whole library streams, on shuffle or repeat. Songs you have heard once are kept on your computer, so they play again instantly.',
             'points' => ['/play, /shuffle, /repeat, /next and /stop', 'Name a song by its number, name or hash', '/save keeps any song as a file']],
            ['n' => '03', 'icon' => 'fa-file-arrow-up', 'title' => 'Upload, share and download files straight from your terminal', 'illustration' => 'files',
             'text' => 'Send a file or a whole folder to a chat with /upload. It is packed and encrypted on your computer before it leaves, with a preview first and a progress bar while it goes. The other side checks its MD5 when it arrives.',
             'points' => ['Tab completes file names as you type', 'Every file has an MD5 hash you can compare', '/download by name, number or the start of the hash']],
        ];
    @endphp

    <div class="space-y-16 sm:space-y-24">
        @foreach ($features as $index => $feature)
            <article class="grid items-center gap-8 lg:grid-cols-2 lg:gap-14">
                <div class="{{ $index % 2 ? 'lg:order-2' : '' }} space-y-4">
                    <p class="flex items-center gap-3 font-mono text-xs uppercase tracking-[0.2em] text-cyber-cyan"><span class="text-gray-600">{{ $feature['n'] }}</span><i class="fa-solid {{ $feature['icon'] }}"></i></p>
                    <h3 class="font-mono text-2xl font-black leading-snug text-white sm:text-3xl">{{ $feature['title'] }}</h3>
                    <p class="text-sm leading-relaxed text-gray-400">{{ $feature['text'] }}</p>
                    <ul class="space-y-2 pt-1 font-mono text-xs text-gray-300">
                        @foreach ($feature['points'] as $point)
                            <li class="flex items-start gap-2.5"><span class="mt-0.5 text-cyber-neon">&gt;</span><span>{{ $point }}</span></li>
                        @endforeach
                    </ul>
                </div>
                <div class="{{ $index % 2 ? 'lg:order-1' : '' }} corners overflow-hidden">
                    @include('partials.illustrations.'.$feature['illustration'])
                </div>
            </article>
        @endforeach
    </div>

    <div class="mt-16 flex flex-col items-center gap-4 border border-cyber-600 bg-cyber-800/90 p-8 text-center sm:mt-24">
        <p class="font-mono text-xl font-black text-white">Ready? It takes a minute.</p>
        <p class="max-w-md text-sm text-gray-400">Make an account, download the client for your system, log in with the same email and password, and you are chatting.</p>
        <div class="flex flex-wrap justify-center gap-3">
            @guest<a href="{{ route('register') }}" class="bg-cyber-neon px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green"><i class="fa-solid fa-user-plus mr-2"></i>Create an account</a>@endguest
            <a href="{{ route('download') }}" class="border border-cyber-600 px-5 py-3 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon"><i class="fa-solid fa-download mr-2"></i>Download</a>
        </div>
    </div>
</section>
@endsection
