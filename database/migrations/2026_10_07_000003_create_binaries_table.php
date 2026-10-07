<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('binaries', function (Blueprint $table) {
            $table->id();
            $table->string('os', 10);          // linux | windows | macos
            $table->string('arch', 10);        // amd64 | arm64
            $table->string('version', 32);
            $table->string('filename');        // the name users download it as
            $table->string('path');            // under storage/app/private
            $table->unsignedBigInteger('size');
            $table->char('md5', 32);
            $table->char('sha256', 64);
            $table->foreignId('uploaded_by')->nullable()->constrained('users')->nullOnDelete();
            $table->timestamps();
            $table->unique(['os', 'arch', 'version']);
        });

        // Register binaries that were already in the downloads folder.
        foreach (['linux' => 'pipechat-linux-amd64', 'windows' => 'pipechat-windows-amd64.exe'] as $os => $file) {
            $path = storage_path("app/private/downloads/$file");
            if (! is_file($path)) {
                continue;
            }
            DB::table('binaries')->insert([
                'os' => $os, 'arch' => 'amd64', 'version' => '0.1.0', 'filename' => $file,
                'path' => "downloads/$file", 'size' => filesize($path),
                'md5' => md5_file($path), 'sha256' => hash_file('sha256', $path),
                'created_at' => now(), 'updated_at' => now(),
            ]);
        }
    }

    public function down(): void
    {
        Schema::dropIfExists('binaries');
    }
};
