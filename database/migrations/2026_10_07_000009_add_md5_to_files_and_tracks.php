<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Every upload gets an MD5, so people can name a file or a song by the start of its hash.
        Schema::table('chat_group_files', function (Blueprint $table) {
            $table->char('md5', 32)->nullable(); // of the file as its sender saw it; the file itself stays encrypted
        });
        Schema::table('tracks', function (Blueprint $table) {
            $table->char('md5', 32)->nullable()->index();
        });

        // Songs uploaded before this get theirs now.
        foreach (DB::table('tracks')->whereNull('md5')->get(['id', 'path']) as $track) {
            $file = storage_path('app/private/'.$track->path);
            if (is_file($file)) {
                DB::table('tracks')->where('id', $track->id)->update(['md5' => md5_file($file)]);
            }
        }
    }

    public function down(): void
    {
        Schema::table('tracks', function (Blueprint $table) {
            $table->dropColumn('md5');
        });
        Schema::table('chat_group_files', function (Blueprint $table) {
            $table->dropColumn('md5');
        });
    }
};
