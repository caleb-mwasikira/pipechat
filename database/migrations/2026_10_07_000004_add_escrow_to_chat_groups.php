<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('chat_groups', function (Blueprint $table) {
            // The group's invite code sealed to the chat server's own key (CHAT_ESCROW_KEY), so new
            // members get the key right away even when no other member is online. Only the chat
            // server can open it; the website never reads this column.
            $table->binary('escrow')->nullable();
        });
    }

    public function down(): void
    {
        Schema::table('chat_groups', function (Blueprint $table) {
            $table->dropColumn('escrow');
        });
    }
};
