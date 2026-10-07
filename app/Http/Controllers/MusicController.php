<?php

namespace App\Http\Controllers;

use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Storage;
use Illuminate\Support\Str;
use Symfony\Component\HttpFoundation\BinaryFileResponse;

class MusicController extends Controller
{
    /** What PHP's file sniffing reports for these formats, so a renamed non-audio file is refused. */
    private const MIMES = ['audio/mpeg', 'audio/mp3', 'audio/mp4', 'audio/x-m4a', 'video/mp4', 'audio/aac', 'audio/x-hx-aac-adts',
        'audio/ogg', 'application/ogg', 'audio/opus', 'audio/wav', 'audio/x-wav', 'audio/wave', 'audio/flac', 'audio/x-flac'];

    public const EXTENSIONS = ['mp3', 'm4a', 'aac', 'ogg', 'oga', 'opus', 'wav', 'flac'];

    public function index()
    {
        return view('music', [
            'tracks' => DB::table('tracks as t')
                ->leftJoin('users as u', 'u.id', '=', 't.uploaded_by')
                ->select('t.*', 'u.handle as uploader')
                ->orderByDesc('t.id')->get(),
        ]);
    }

    public function store(Request $request): RedirectResponse
    {
        $data = $request->validate([
            'file' => ['required', 'file', 'max:51200', 'extensions:'.implode(',', self::EXTENSIONS), 'mimetypes:'.implode(',', self::MIMES)],
            'title' => ['nullable', 'string', 'max:120'],
            'artist' => ['nullable', 'string', 'max:120'],
        ], ['file.max' => 'Music files can be at most 50 MB.', 'file.mimetypes' => 'That does not look like an audio file.', 'file.extensions' => 'Upload '.strtoupper(implode(', ', self::EXTENSIONS)).' files only.']);

        $file = $request->file('file');
        $extension = strtolower($file->getClientOriginalExtension());
        $path = $file->storeAs('music', Str::uuid().'.'.$extension);
        $name = pathinfo($file->getClientOriginalName(), PATHINFO_FILENAME);

        DB::table('tracks')->insert([
            'title' => trim($data['title'] ?? '') ?: Str::limit(str_replace(['_', '-'], ' ', $name), 120, ''),
            'artist' => trim($data['artist'] ?? '') ?: null,
            'path' => $path, 'extension' => $extension, 'size' => $file->getSize(),
            'uploaded_by' => $request->user()->id, 'created_at' => now(), 'updated_at' => now(),
        ]);

        return back()->with('status', 'Track uploaded. Play it in the terminal with /music and /play.');
    }

    /** The browser's own player. The terminal client gets the file from the chat server instead. */
    public function audio(int $track): BinaryFileResponse
    {
        $row = DB::table('tracks')->find($track) ?? abort(404);
        $path = storage_path('app/private/'.$row->path);
        abort_unless(is_file($path), 404);

        return response()->file($path, ['Content-Type' => $this->mime($row->extension)]);
    }

    public function destroy(Request $request, int $track): RedirectResponse
    {
        $row = DB::table('tracks')->find($track) ?? abort(404);
        // People remove their own tracks; an admin can remove any.
        abort_unless($request->user()->is_admin || $row->uploaded_by === $request->user()->id, 403);

        DB::table('tracks')->where('id', $track)->delete();
        Storage::delete($row->path);

        return back()->with('status', "Deleted {$row->title}.");
    }

    private function mime(string $extension): string
    {
        return ['mp3' => 'audio/mpeg', 'm4a' => 'audio/mp4', 'aac' => 'audio/aac', 'ogg' => 'audio/ogg', 'oga' => 'audio/ogg',
            'opus' => 'audio/ogg', 'wav' => 'audio/wav', 'flac' => 'audio/flac'][$extension] ?? 'application/octet-stream';
    }
}
