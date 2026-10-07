<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="csrf-token" content="{{ csrf_token() }}">
    <title>@hasSection('title')@yield('title') // @endif{{ config('app.name') }} // Terminal chat</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    @vite(['resources/css/app.css', 'resources/js/app.js'])
</head>
<body class="min-h-screen bg-cyber-900 flex flex-col relative selection:bg-cyber-neon selection:text-black">
    <div class="fixed inset-0 scanlines z-50 pointer-events-none"></div>

    <div class="hidden border-b border-cyber-700/60 bg-cyber-900 sm:block">
        <div class="mx-auto flex h-7 w-full max-w-screen-2xl items-center justify-between px-4 font-mono text-[10px] uppercase tracking-[0.2em] text-gray-500 sm:px-6 lg:px-8">
            <span class="flex items-center gap-2"><span class="h-1.5 w-1.5 animate-pulse bg-cyber-neon"></span>secure channel &middot; online</span>
            <span>end-to-end encrypted</span>
        </div>
    </div>

    <header class="sticky top-0 z-40 border-b border-cyber-600/60 bg-cyber-900/85 backdrop-blur-md">
        <div class="mx-auto flex h-16 w-full max-w-screen-2xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
            <a href="{{ route('home') }}" class="group flex shrink-0 items-center gap-2.5" aria-label="{{ config('app.name') }} home">
                <span class="flex h-9 w-9 shrink-0 items-center justify-center border border-cyber-neon bg-cyber-800 font-mono text-sm font-black text-cyber-neon transition-all group-hover:bg-cyber-neon group-hover:text-black">&gt;_</span>
                <span class="font-mono text-lg font-black tracking-tight text-white">{{ config('app.name') }}<span class="text-cyber-neon">.</span></span>
            </a>

            <div class="flex shrink-0 items-center gap-2 sm:gap-3">
                @php $navClass = fn ($active) => 'whitespace-nowrap border-b-2 px-2 py-5 font-mono text-xs font-bold uppercase tracking-wider transition-colors '.($active ? 'border-cyber-neon text-cyber-neon' : 'border-transparent text-gray-400 hover:text-white'); @endphp
                @auth<a href="{{ route('groups') }}" class="{{ $navClass(request()->routeIs('groups')) }}">Groups</a>@endauth
                <a href="{{ route('download') }}" class="{{ $navClass(request()->routeIs('download')) }}">Download</a>
                @auth
                    @if (auth()->user()->is_admin)<a href="{{ route('admin') }}" class="{{ $navClass(request()->routeIs('admin')) }}">Admin</a>@endif
                @endauth
                @guest
                    <a href="{{ route('login') }}" class="whitespace-nowrap border border-cyber-600 px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-gray-300 transition-all hover:border-cyber-neon hover:text-cyber-neon">Login</a>
                    <a href="{{ route('register') }}" class="whitespace-nowrap bg-cyber-neon px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-black transition-all hover:bg-emerald-400 neon-glow-green">Register</a>
                @else
                    <span class="hidden max-w-[10rem] truncate font-mono text-xs text-gray-400 sm:inline"><i class="fa-solid fa-user-secret mr-1.5 text-cyber-cyan"></i>{{ auth()->user()->name }}</span>
                    <form method="POST" action="{{ route('logout') }}">
                        @csrf
                        <button type="submit" class="whitespace-nowrap border border-cyber-600 px-3 py-2 font-mono text-xs font-bold uppercase tracking-wider text-red-400 transition-all hover:bg-red-500 hover:text-black">Logout</button>
                    </form>
                @endguest
            </div>
        </div>
    </header>

    <main class="flex-grow @yield('container', 'max-w-5xl') mx-auto px-3 sm:px-6 lg:px-8 py-8 w-full">
        @yield('content')
    </main>

    <footer class="border-t border-cyber-600/60 bg-cyber-900">
        <div class="mx-auto flex w-full max-w-screen-2xl flex-col items-center justify-between gap-2 px-4 py-6 font-mono text-[11px] text-gray-500 sm:flex-row sm:px-6 lg:px-8">
            <span>&copy; {{ date('Y') }} {{ config('app.name') }}</span>
            <span><span class="text-cyber-neon">$</span> exit 0</span>
        </div>
    </footer>
</body>
</html>
