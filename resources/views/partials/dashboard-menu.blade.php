@php
    $user = auth()->user();
    $item = fn ($active) => 'flex items-center gap-2.5 border-l-2 px-4 py-3 font-mono text-xs font-bold transition-all '.($active ? 'border-cyber-neon bg-cyber-700/60 text-cyber-neon' : 'border-transparent text-gray-300 hover:border-cyber-neon hover:bg-cyber-700/50 hover:text-white');
    $label = 'px-4 pt-4 pb-1 font-mono text-[10px] font-bold uppercase tracking-wider';
@endphp
<nav class="py-3" aria-label="Dashboard">
    <div class="px-4 pb-3 border-b border-cyber-700">
        <p class="truncate font-mono text-sm font-bold text-white"><i class="fa-solid fa-user-secret mr-1.5 text-cyber-cyan"></i>{{ $user->name }}</p>
        <p class="truncate font-mono text-[11px] text-gray-400">{{ '@'.$user->handle }}</p>
    </div>

    <p class="{{ $label }} text-gray-500">Account</p>
    <a href="{{ route('groups') }}" class="{{ $item(request()->routeIs('groups')) }}"><i class="fa-solid fa-users w-4 text-center"></i>Groups</a>
    <a href="{{ route('download') }}" class="{{ $item(false) }}"><i class="fa-solid fa-download w-4 text-center"></i>Download</a>

    @if ($user->is_admin)
        <p class="{{ $label }} text-cyber-purple"><i class="fa-solid fa-user-shield mr-1"></i>Admin</p>
        <a href="{{ route('admin') }}" class="{{ $item(request()->routeIs('admin')) }}"><i class="fa-solid fa-chart-line w-4 text-center"></i>Stats</a>
        <a href="{{ route('admin.binaries') }}" class="{{ $item(request()->routeIs('admin.binaries')) }}"><i class="fa-solid fa-box-archive w-4 text-center"></i>Binaries</a>
        <a href="{{ route('admin.binaries.create') }}" class="{{ $item(request()->routeIs('admin.binaries.create')) }}"><i class="fa-solid fa-upload w-4 text-center"></i>Upload binary</a>
    @endif

    <form method="POST" action="{{ route('logout') }}" class="mt-3 border-t border-cyber-700 pt-2">
        @csrf
        <button type="submit" class="flex w-full items-center gap-2.5 border-l-2 border-transparent px-4 py-3 text-left font-mono text-xs font-bold text-red-400 transition-all hover:bg-red-500 hover:text-black"><i class="fa-solid fa-power-off w-4 text-center"></i>Logout</button>
    </form>
</nav>
