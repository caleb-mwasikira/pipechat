<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('users', function (Blueprint $table) {
            // The name shown in the terminal chat: 3-20 characters of a-z, 0-9 and _.
            $table->string('handle', 20)->nullable()->unique();
        });

        // Give existing users a handle made from their email.
        $taken = [];
        foreach (DB::table('users')->orderBy('id')->get(['id', 'email']) as $user) {
            $base = substr(preg_replace('/[^a-z0-9_]/', '', strtolower(strstr($user->email, '@', true))), 0, 16);
            $base = str_pad($base, 3, '0');
            $handle = $base;
            for ($n = 2; in_array($handle, $taken); $n++) {
                $handle = $base.$n;
            }
            $taken[] = $handle;
            DB::table('users')->where('id', $user->id)->update(['handle' => $handle]);
        }
    }

    public function down(): void
    {
        Schema::table('users', function (Blueprint $table) {
            $table->dropColumn('handle');
        });
    }
};
