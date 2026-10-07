<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;
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

    /**
     * Upload one or many songs. The page sends them one at a time (so it can show progress); a form without
     * script can send several at once as files[]. Each song is checked on its own: one bad file does not stop the rest.
     */
    public function store(Request $request): RedirectResponse|JsonResponse
    {
        $rules = ['title' => ['nullable', 'string', 'max:120'], 'artist' => ['nullable', 'string', 'max:120']];
        $fileRule = ['file', 'max:51200', 'extensions:'.implode(',', self::EXTENSIONS), 'mimetypes:'.implode(',', self::MIMES)];
        $messages = ['*.max' => 'Music files can be at most 50 MB.', '*.mimetypes' => 'That does not look like an audio file.',
            '*.extensions' => 'Upload '.strtoupper(implode(', ', self::EXTENSIONS)).' files only.'];

        $uploads = collect($request->file('files', []))->push($request->file('file'))->filter()->values();
        abort_if($uploads->isEmpty(), 422, 'Choose at least one audio file.');
        abort_if($uploads->count() > 200, 422, 'Upload at most 200 files at a time.');

        $saved = [];
        $skipped = [];
        foreach ($uploads as $file) {
            $check = validator(['file' => $file] + $request->only('title', 'artist'), $rules + ['file' => ['required', ...$fileRule]], $messages);
            if ($check->fails()) {
                if ($request->expectsJson()) {
                    return response()->json(['message' => $file->getClientOriginalName().': '.$check->errors()->first(), 'errors' => $check->errors()], 422);
                }
                $skipped[] = $file->getClientOriginalName().': '.$check->errors()->first();
                continue;
            }

            // The same song is not stored twice.
            $md5 = md5_file($file->getRealPath());
            if ($existing = DB::table('tracks')->where('md5', $md5)->first()) {
                if ($request->expectsJson()) {
                    return response()->json(['message' => "{$file->getClientOriginalName()} is already in the library as #{$existing->id}.", 'duplicate' => $existing->id], 409);
                }
                $skipped[] = "{$file->getClientOriginalName()} is already in the library as #{$existing->id}";
                continue;
            }

            $extension = strtolower($file->getClientOriginalExtension());
            $path = $file->storeAs('music', Str::uuid().'.'.$extension);
            [$artist, $title] = $this->namesFor($file->getClientOriginalName(), count($uploads) === 1 ? $request->input('title') : null, $request->input('artist'));

            $id = DB::table('tracks')->insertGetId([
                'title' => $title, 'artist' => $artist, 'path' => $path, 'extension' => $extension, 'size' => $file->getSize(),
                'md5' => $md5, 'uploaded_by' => $request->user()->id, 'created_at' => now(), 'updated_at' => now(),
            ]);
            $saved[] = ['id' => $id, 'title' => $title];
        }

        if ($request->expectsJson()) {
            return response()->json($saved[0] ?? [], 201);
        }
        $message = count($saved) === 1 ? 'Track uploaded.' : count($saved).' tracks uploaded.';
        if ($skipped) {
            $message .= ' Skipped: '.implode('; ', array_slice($skipped, 0, 5)).(count($skipped) > 5 ? '; and '.(count($skipped) - 5).' more' : '');
        }

        return back()->with($saved ? 'status' : 'error', $saved ? $message.' Play them in the terminal with /music and /play.' : $message);
    }

    /** Artist and title for a song: what was typed, else taken from a file name like "Artist - Title.mp3". */
    private function namesFor(string $fileName, ?string $typedTitle, ?string $typedArtist): array
    {
        $name = trim(str_replace('_', ' ', pathinfo($fileName, PATHINFO_FILENAME)));
        $artist = null;
        if (str_contains($name, ' - ')) {
            [$artist, $name] = array_map('trim', explode(' - ', $name, 2));
        }
        $title = trim($typedTitle ?? '') ?: Str::limit($name, 120, '');

        return [trim($typedArtist ?? '') ?: ($artist ? Str::limit($artist, 120, '') : null), $title];
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
