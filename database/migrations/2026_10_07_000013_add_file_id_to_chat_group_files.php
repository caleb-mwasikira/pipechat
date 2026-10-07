<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Which stored file a listed share is, so the group's owner can delete it from the terminal (/delete).
        Schema::table('chat_group_files', function (Blueprint $table) {
            $table->string('file_id', 32)->nullable()->index()->after('md5');
        });
    }

    public function down(): void
    {
        Schema::table('chat_group_files', function (Blueprint $table) {
            $table->dropColumn('file_id');
        });
    }
};
