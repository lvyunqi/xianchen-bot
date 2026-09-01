#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

typedef char *(__stdcall *BeeAPIFn)(char *command);
typedef const char *(__stdcall *InitFn)(const char *);
typedef int (__stdcall *GroupMessageFn)(const char *, const char *, const char *, const char *, const char *);
typedef int (__stdcall *PrivateMessageFn)(const char *, const char *, const char *, const char *);
typedef void (__stdcall *LifecycleFn)(const char *);

static int group_sends;
static int friend_sends;
static int group_markdown_sends;
static int friend_markdown_sends;

static char *__stdcall fake_bee_api(char *command) {
    int opcode = command ? atoi(command) : 0;
    static char ok[] = "message-id";
    if (opcode == 34) group_sends++;
    if (opcode == 52) friend_sends++;
    if (opcode == 38) group_markdown_sends++;
    if (opcode == 55) friend_markdown_sends++;
    return ok;
}

int main(int argc, char **argv) {
    static const char init_export[] = "Bee_\xb3\xf5\xca\xbc\xbb\xaf";
    static const char enable_export[] = "Bee_\xb2\xe5\xbc\xfe\xb1\xbb\xc6\xf4\xd3\xc3";
    static const char disable_export[] = "Bee_\xb2\xe5\xbc\xfe\xb1\xbb\xbd\xfb\xd3\xc3";
    static const char group_export[] = "Bee_\xca\xd5\xb5\xbd\xc8\xba\xc1\xc4\xcf\xfb\xcf\xa2";
    static const char private_export[] = "Bee_\xca\xd5\xb5\xbd\xcb\xbd\xc1\xc4\xcf\xfb\xcf\xa2";
    static const char unload_export[] = "Bee_\xb2\xe5\xbc\xfe\xb1\xbb\xd0\xb6\xd4\xd8";
    static const char group_command[] = "\xbb\xf1\xc8\xa1\xc8\xbaID";
    static const char private_command[] = "\xbb\xf1\xc8\xa1ID";
    char robot[256];
    HMODULE module;
    InitFn initialize;
    LifecycleFn enable;
    LifecycleFn disable;
    GroupMessageFn group_message;
    PrivateMessageFn private_message;
    LifecycleFn unload;
    const char *metadata;
    int group_result;
    int private_result;

    if (argc != 2) {
        fprintf(stderr, "usage: bee_host.exe <plugin.dll>\n");
        return 2;
    }
    module = LoadLibraryA(argv[1]);
    if (!module) {
        fprintf(stderr, "LoadLibrary failed: %lu\n", GetLastError());
        return 3;
    }
    initialize = (InitFn)GetProcAddress(module, init_export);
    enable = (LifecycleFn)GetProcAddress(module, enable_export);
    disable = (LifecycleFn)GetProcAddress(module, disable_export);
    group_message = (GroupMessageFn)GetProcAddress(module, group_export);
    private_message = (PrivateMessageFn)GetProcAddress(module, private_export);
    unload = (LifecycleFn)GetProcAddress(module, unload_export);
    if (!initialize || !enable || !disable || !group_message || !private_message || !unload) {
        fprintf(stderr, "plugin exports missing: init=%p enable=%p disable=%p group=%p private=%p unload=%p\n",
                initialize, enable, disable, group_message, private_message, unload);
        FreeLibrary(module);
        return 4;
    }
    snprintf(robot, sizeof(robot), "{\"api\":\"%lu\",\"plugin_id\":\"host-test\",\"msg_id\":\"host-message\"}",
             (unsigned long)(uintptr_t)(BeeAPIFn)fake_bee_api);
	printf("robot=%s\n", robot);

    metadata = initialize(robot);
    if (!metadata || !metadata[0] || !strstr(metadata, "2.2.2")) {
        fprintf(stderr, "initialization metadata is empty or has the wrong version\n");
        FreeLibrary(module);
        return 5;
    }
    enable(robot);
    group_result = group_message(robot, "group-test", "user-test", group_command, "group-message-id");
    private_result = private_message(robot, "friend-test", private_command, "private-message-id");
    disable(robot);
    unload(robot);
    FreeLibrary(module);

    printf("group_result=%d private_result=%d group_text=%d friend_text=%d group_markdown=%d friend_markdown=%d\n",
           group_result, private_result, group_sends, friend_sends, group_markdown_sends, friend_markdown_sends);
    if (group_result != 1 || private_result != 1 ||
        group_sends + group_markdown_sends < 1 || friend_sends + friend_markdown_sends < 1) return 6;
    return 0;
}
