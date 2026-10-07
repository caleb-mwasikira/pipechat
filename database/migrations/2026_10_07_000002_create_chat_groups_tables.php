<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

/*
 * Groups made on the website. The website never learns a group's encryption key:
 * it only keeps names, members and invitations. The chat server and the members'
 * terminals use these tables to create the group and hand its key (sealed to each
 * member's public key) from one member to the next.
 */
return new class extends Migration
{
    public function up(): void
    {
        Schema::create('chat_groups', function (Blueprint $table) {
            $table->id();
            $table->string('name', 24);
            $table->foreignId('owner_id')->constrained('users')->cascadeOnDelete();
            $table->string('gid', 16)->nullable()->unique(); // set once the owner's terminal has created the group
            $table->timestamps();
        });

        Schema::create('chat_group_members', function (Blueprint $table) {
            $table->id();
            $table->foreignId('group_id')->constrained('chat_groups')->cascadeOnDelete();
            $table->foreignId('user_id')->constrained('users')->cascadeOnDelete();
            $table->boolean('delivered')->default(false); // their terminal has the group key
            $table->timestamps();
            $table->unique(['group_id', 'user_id']);
        });

        Schema::create('chat_group_invites', function (Blueprint $table) {
            $table->id();
            $table->foreignId('group_id')->constrained('chat_groups')->cascadeOnDelete();
            $table->foreignId('inviter_id')->constrained('users')->cascadeOnDelete();
            $table->foreignId('invitee_id')->constrained('users')->cascadeOnDelete();
            $table->string('status', 10)->default('pending'); // pending | accepted | declined
            $table->timestamps();
            $table->unique(['group_id', 'invitee_id']);
        });

        Schema::create('chat_group_grants', function (Blueprint $table) {
            $table->foreignId('group_id')->constrained('chat_groups')->cascadeOnDelete();
            $table->foreignId('user_id')->constrained('users')->cascadeOnDelete();
            $table->foreignId('from_user_id')->constrained('users')->cascadeOnDelete();
            $table->binary('sealed'); // the group's invite code, sealed to the recipient's public key
            $table->timestamp('created_at')->nullable();
            $table->primary(['group_id', 'user_id']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('chat_group_grants');
        Schema::dropIfExists('chat_group_invites');
        Schema::dropIfExists('chat_group_members');
        Schema::dropIfExists('chat_groups');
    }
};
