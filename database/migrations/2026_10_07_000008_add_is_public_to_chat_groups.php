<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('chat_groups', function (Blueprint $table) {
            // Private: people need an invitation. Public: anyone with an account can join from the list.
            $table->boolean('is_public')->default(false);
        });

        // A public group is found by its name, so two public groups cannot share one (private ones can).
        DB::statement('CREATE UNIQUE INDEX chat_groups_public_name_unique ON chat_groups (lower(name)) WHERE is_public');
    }

    public function down(): void
    {
        DB::statement('DROP INDEX IF EXISTS chat_groups_public_name_unique');
        Schema::table('chat_groups', function (Blueprint $table) {
            $table->dropColumn('is_public');
        });
    }
};
