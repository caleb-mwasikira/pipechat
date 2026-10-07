<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Files members shared in a website group, as the group's page lists them. Only the name and size
        // are known here: the file itself is encrypted and its key is only in the link posted in the chat.
        Schema::create('chat_group_files', function (Blueprint $table) {
            $table->id();
            $table->foreignId('group_id')->constrained('chat_groups')->cascadeOnDelete();
            $table->foreignId('user_id')->nullable()->constrained('users')->nullOnDelete();
            $table->string('name', 80);
            $table->unsignedBigInteger('size');
            $table->timestamp('expires_at')->nullable(); // the chat server deletes the file after this
            $table->timestamp('created_at')->nullable();
            $table->index(['group_id', 'created_at']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('chat_group_files');
    }
};
