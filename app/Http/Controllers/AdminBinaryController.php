<?php

namespace App\Http\Controllers;

use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;
use Illuminate\Validation\Rule;

class AdminBinaryController extends Controller
{
    public function index()
    {
        return view('admin-binaries', [
            'binaries' => DB::table('binaries')->orderBy('os')->orderBy('arch')->get()
                ->sort(fn ($a, $b) => version_compare($b->version, $a->version))
                ->sortBy(fn ($b) => $b->os.$b->arch),
        ]);
    }

    public function store(Request $request): RedirectResponse
    {
        $data = $request->validate([
            'os' => ['required', Rule::in(array_keys(DownloadController::OSES))],
            'arch' => ['required', Rule::in(['amd64', 'arm64'])],
            'version' => ['required', 'regex:/^\d+\.\d+\.\d+(-[A-Za-z0-9.]+)?$/', 'max:32',
                Rule::unique('binaries')->where(fn ($q) => $q->where('os', $request->os)->where('arch', $request->arch))],
            'file' => ['required', 'file'],
        ], ['version.regex' => 'Use a version like 1.2.3 or 1.2.3-beta.1.',
            'version.unique' => 'That OS, architecture and version already has a binary.']);

        $file = $request->file('file');
        // The user downloads it under a predictable name, not whatever it was called on the admin's disk.
        $filename = sprintf('pipechat-%s-%s-%s%s', $data['version'], $data['os'], $data['arch'], $data['os'] === 'windows' ? '.exe' : '');
        $path = $file->storeAs('downloads', $filename);

        DB::table('binaries')->insert([
            'os' => $data['os'], 'arch' => $data['arch'], 'version' => $data['version'],
            'filename' => $filename, 'path' => $path, 'size' => $file->getSize(),
            'md5' => md5_file($file->getRealPath()), 'sha256' => hash_file('sha256', $file->getRealPath()),
            'uploaded_by' => $request->user()->id, 'created_at' => now(), 'updated_at' => now(),
        ]);

        return back()->with('status', "Uploaded $filename.");
    }

    public function destroy(int $binary): RedirectResponse
    {
        $row = DB::table('binaries')->find($binary) ?? abort(404);
        DB::table('binaries')->where('id', $binary)->delete();
        // Two rows never share a file, but keep a file that is still listed.
        if (! DB::table('binaries')->where('path', $row->path)->exists()) {
            Storage::delete($row->path);
        }

        return back()->with('status', "Deleted {$row->filename}.");
    }
}
