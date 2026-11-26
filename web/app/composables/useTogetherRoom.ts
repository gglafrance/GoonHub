import type { InjectionKey } from 'vue';
import type { LobbyMember, QueueItem } from '~/types/together';

/**
 * A Goon2gether room: the socket plus what the signed-in user may do in it, computed the way
 * the server decides (docs/GOON2GETHER.md "Concepts"). Provided by the room page to its
 * components (inject with TOGETHER_ROOM_KEY).
 */
export const useTogetherRoom = (lobbyId: Ref<string>) => {
    const socket = useTogetherSocket(lobbyId);
    const perms = useMyPermissions();

    const myId = computed(() => socket.you.value?.user_id ?? null);
    const me = computed<LobbyMember | null>(
        () => socket.members.value.find((m) => m.user_id === myId.value) ?? null,
    );
    const isHost = computed(
        () => myId.value !== null && socket.lobby.value?.host_id === myId.value,
    );
    const canControl = computed(
        () => isHost.value || !!socket.lobby.value?.control_open || !!me.value?.control,
    );
    const canQueue = computed(() => canControl.value || !!socket.lobby.value?.queue_open);
    const canModerate = computed(() => !!socket.you.value?.moderate);
    const canAddScenes = computed(() => perms.has('scenes:view'));

    // queue[0] is the item playing when the state has one
    const currentItem = computed<QueueItem | null>(() => {
        const id = socket.state.value?.item_id;
        if (id == null) return null;
        return socket.queue.value.find((i) => i.id === id) ?? null;
    });
    const upcoming = computed(() =>
        socket.queue.value.filter((i) => i.id !== socket.state.value?.item_id),
    );

    const onlineCount = computed(() => socket.members.value.filter((m) => m.online).length);

    return {
        socket,
        myId,
        me,
        isHost,
        canControl,
        canQueue,
        canModerate,
        canAddScenes,
        currentItem,
        upcoming,
        onlineCount,
    };
};

export type TogetherRoom = ReturnType<typeof useTogetherRoom>;

export const TOGETHER_ROOM_KEY: InjectionKey<TogetherRoom> = Symbol('together-room');

/** The room provided by the room page. */
export const useInjectedTogetherRoom = (): TogetherRoom => {
    const room = inject(TOGETHER_ROOM_KEY);
    if (!room) throw new Error('useInjectedTogetherRoom outside of a Goon2gether room');
    return room;
};
