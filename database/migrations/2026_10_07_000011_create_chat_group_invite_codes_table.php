<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Short codes a group's creator hands out. In the terminal, /join CODE puts the person in the group (the
        // chat server does that, reading and updating this table). A code stops working when it expires, is used up,
        // or is revoked (deleted).
        Schema::create('chat_group_invite_codes', function (Blueprint $table) {
            $table->id();
            $table->foreignId('group_id')->constrained('chat_groups')->cascadeOnDelete();
            $table->foreignId('created_by')->nullable()->constrained('users')->nullOnDelete();
            $table->string('code', 12)->unique();          // upper case, without the dash
            $table->unsignedInteger('uses')->default(0);
            $table->unsignedInteger('max_uses')->default(25);
            $table->timestamp('expires_at');
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('chat_group_invite_codes');
    }
};
