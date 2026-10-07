<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;
use Illuminate\Support\Str;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('chat_groups', function (Blueprint $table) {
            // /join/{token}: anyone who opens it while logged in can join the group. The creator can turn it
            // off (null) or replace it, which makes the old link stop working.
            $table->string('invite_token', 40)->nullable()->unique();
        });

        foreach (DB::table('chat_groups')->pluck('id') as $id) {
            DB::table('chat_groups')->where('id', $id)->update(['invite_token' => Str::random(32)]);
        }
    }

    public function down(): void
    {
        Schema::table('chat_groups', function (Blueprint $table) {
            $table->dropColumn('invite_token');
        });
    }
};
