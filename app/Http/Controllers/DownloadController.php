<?php

namespace App\Http\Controllers;

use Illuminate\Support\Facades\DB;
use Symfony\Component\HttpFoundation\BinaryFileResponse;

class DownloadController extends Controller
{
    public const OSES = ['linux' => 'Linux', 'windows' => 'Windows', 'macos' => 'macOS'];

    public function index()
    {
        // Newest version first within each OS and architecture.
        $builds = DB::table('binaries')->get()
            ->sort(fn ($a, $b) => version_compare($b->version, $a->version))
            ->groupBy(fn ($b) => $b->os.'/'.$b->arch)
            ->sortKeys();

        return view('download', ['builds' => $builds]);
    }

    public function show(int $binary): BinaryFileResponse
    {
        $row = DB::table('binaries')->find($binary) ?? abort(404);
        $path = storage_path('app/private/'.$row->path);
        abort_unless(is_file($path), 404);

        return response()->download($path, $row->filename, [
            'X-Checksum-MD5' => $row->md5,
            'X-Checksum-SHA256' => $row->sha256,
        ]);
    }
}
