import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { getUserPTProfile, type UserPTProfile } from "@/api/pt";

/** 本地存储的用户令牌键名 */
export const TOKEN_KEY = 'PAOPAO_TOKEN';

export const useStoreUser = defineStore('user', () => {
    const userInfo = ref<Record<string, any>>({
        id: 0,
        username: '',
        nickname: '',
        created_on: 0,
        follows: 0,
        followings: 0,
        tweets_count: 0,
        is_admin: false,
    });

    const userPTProfile = ref<UserPTProfile | null>(null);
    let pendingProfilePromise: Promise<UserPTProfile | null> | null = null;

    const userLogined = computed(() => userInfo.value.id > 0);

    function updateUserinfo(data: Record<string, any>) {
        userInfo.value = data;
        if (data?.id > 0) {
            loadUserPTProfile();
        }
    }

    async function loadUserPTProfile(force = false): Promise<UserPTProfile | null> {
        if (!userLogined.value) {
            userPTProfile.value = null;
            return null;
        }
        if (userPTProfile.value && !force) {
            return userPTProfile.value;
        }
        if (pendingProfilePromise && !force) {
            return pendingProfilePromise;
        }
        pendingProfilePromise = getUserPTProfile()
            .then((res) => {
                userPTProfile.value = res;
                pendingProfilePromise = null;
                return res;
            })
            .catch(() => {
                pendingProfilePromise = null;
                return null;
            });
        return pendingProfilePromise;
    }

    function userLogout() {
        localStorage.removeItem(TOKEN_KEY);
        userInfo.value = {
            id: 0,
            nickname: '',
            username: '',
            created_on: 0,
            follows: 0,
            followings: 0,
            tweets_count: 0,
            is_admin: false,
        };
        userPTProfile.value = null;
        pendingProfilePromise = null;
    }

    return {
        userInfo,
        userLogined,
        userPTProfile,
        loadUserPTProfile,
        updateUserinfo,
        userLogout,
    }
});