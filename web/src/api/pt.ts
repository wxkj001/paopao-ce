import { request } from '@/utils/request';
import { TOKEN_KEY } from '@/store/user';

export interface PTCategory {
	id: number;
	name: string;
	icon: string;
	sort: number;
}

export interface TorrentItem {
	id: number;
	info_hash: string;
	name: string;
	clean_name: string;
	small_descr: string;
	descr: string;
	category_id: number;
	size: number;
	num_files: number;
	user_id: number;
	username?: string;
	nickname?: string;
	sp_state: number; // 1:Normal, 2:Free, 3:2X, 4:2XFree, 5:50%, 6:2X50%, 7:30%
	promotion_until: number;
	seeders: number;
	leechers: number;
	times_completed: number;
	hits: number;
	is_anonymous: boolean;
	status: number;
	nfo?: string;
	created_at: number;
	updated_at: number;
	category?: PTCategory;
}

export interface TorrentFile {
	id: number;
	torrent_id: number;
	file_path: string;
	size: number;
}

export interface TorrentPeer {
	id: number;
	torrent_id: number;
	user_id: number;
	peer_id: string;
	ip: string;
	port: number;
	uploaded: number;
	downloaded: number;
	to_go: number;
	is_seeder: boolean;
	agent: string;
	last_action: number;
}

export interface NextClassProgress {
	next_class: number;
	next_class_name: string;
	req_weeks: number;
	cur_weeks: number;
	req_upload_gb: number;
	cur_upload_gb: number;
	req_download_gb: number;
	cur_download_gb: number;
	req_ratio: number;
	cur_ratio: number;
	req_bonus: number;
	cur_bonus: number;
}

export interface UserPTProfile {
	user_id: number;
	passkey: string;
	uploaded: number;
	downloaded: number;
	seed_bonus: number;
	seed_time: number;
	leech_time: number;
	can_leech: boolean;
	can_upload: boolean;
	invites: number;
	ratio: number;
	seeding_count: number;
	leech_count: number;
	http_tracker_url?: string;
	ws_tracker_url?: string;
	class: number;
	max_class_once: number;
	vip_until: number;
	custom_title: string;
	class_name: string;
	class_color: string;
	privileges?: string[];
	karma_multiplier?: number;
	can_custom_promotion?: boolean;
	can_anonymous?: boolean;
	can_download?: boolean;
	next_class?: NextClassProgress;
}

export interface BonusOption {
	id: number;
	name: string;
	points: number;
	upload_bytes?: number;
	invites?: number;
	description: string;
}

export interface TorrentListResp {
	total: number;
	list: TorrentItem[];
}

export const getPTCategories = (): Promise<PTCategory[]> => {
	return request({
		method: 'get',
		url: '/v1/pt/categories',
	});
};

export const getTorrents = (params: any): Promise<TorrentListResp> => {
	return request({
		method: 'get',
		url: '/v1/pt/torrents',
		params,
	});
};

export const getTorrentDetail = (id: number): Promise<{
	torrent: TorrentItem;
	files: TorrentFile[];
	peers: TorrentPeer[];
}> => {
	return request({
		method: 'get',
		url: '/v1/pt/torrent',
		params: { id },
	});
};

export const uploadTorrent = (formData: FormData): Promise<TorrentItem> => {
	return request({
		method: 'post',
		url: '/v1/pt/torrent/upload',
		data: formData,
		headers: {
			'Content-Type': 'multipart/form-data',
		},
	});
};

export const editTorrent = (data: any): Promise<TorrentItem> => {
	return request({
		method: 'post',
		url: '/v1/pt/torrent/edit',
		data,
	});
};

export const deleteTorrent = (id: number): Promise<void> => {
	return request({
		method: 'post',
		url: '/v1/pt/torrent/delete',
		data: { id },
	});
};

export const getUserPTProfile = (userId?: number): Promise<UserPTProfile> => {
	return request({
		method: 'get',
		url: '/v1/pt/user/profile',
		params: userId ? { user_id: userId } : undefined,
	});
};

export const resetUserPasskey = (): Promise<{ passkey: string }> => {
	return request({
		method: 'post',
		url: '/v1/pt/user/passkey/reset',
	});
};

export const getUserSnatches = (params: { page: number; page_size: number }): Promise<{ total: number; list: any[] }> => {
	return request({
		method: 'get',
		url: '/v1/pt/user/snatches',
		params,
	});
};

export const getBonusOptions = (): Promise<BonusOption[]> => {
	return request({
		method: 'get',
		url: '/v1/pt/bonus/options',
	});
};

export const exchangeBonus = (option_id: number): Promise<void> => {
	return request({
		method: 'post',
		url: '/v1/pt/bonus/exchange',
		data: { option_id },
	});
};

export const getTorrentDownloadURL = (id: number, passkey?: string): string => {
	const host = import.meta.env.VITE_HOST || '';
	const token = localStorage.getItem(TOKEN_KEY) || '';
	if (passkey) {
		return `${host}/v1/pt/torrent/download?id=${id}&passkey=${passkey}`;
	}
	return `${host}/v1/pt/torrent/download?id=${id}&token=${token}`;
};

export const getTorrentStreamURL = (id: number, passkey?: string): string => {
	const host = import.meta.env.VITE_HOST || '';
	const token = localStorage.getItem(TOKEN_KEY) || '';
	if (passkey) {
		return `${host}/v1/pt/torrent/stream?id=${id}&passkey=${passkey}`;
	}
	return `${host}/v1/pt/torrent/stream?id=${id}&token=${token}`;
};

export const getSpLabel = (sp?: number): string => {
	switch (sp) {
		case 1: return '普通';
		case 2: return '免费 (Free)';
		case 3: return '2X 上传';
		case 4: return '2X 免费';
		case 5: return '50% 下载';
		case 6: return '2X 50%';
		case 7: return '30% 下载';
		default: return '普通';
	}
};

export const getSpType = (sp?: number): 'info' | 'warning' | 'success' | 'error' | 'default' => {
	switch (sp) {
		case 1: return 'default';
		case 2: return 'success';
		case 3: return 'warning';
		case 4: return 'error';
		case 5:
		case 6:
		case 7: return 'info';
		default: return 'default';
	}
};

export interface AdminPTUserItem {
	user_id: number;
	username: string;
	nickname: string;
	avatar: string;
	is_admin: boolean;
	class: number;
	class_name: string;
	class_color: string;
	custom_title: string;
	vip_until: number;
	uploaded: number;
	downloaded: number;
	ratio: number;
	seed_bonus: number;
	can_leech: boolean;
	can_upload: boolean;
	created_on: number;
}

export interface AdminClassOption {
	class: number;
	name: string;
	color: string;
}

export const adminGetClassOptions = (): Promise<AdminClassOption[]> => {
	return request({
		method: 'get',
		url: '/v1/pt/admin/classes',
	});
};

export const adminGetUsers = (params: { keyword?: string; class?: number; page?: number; page_size?: number }): Promise<{ total: number; list: AdminPTUserItem[] }> => {
	return request({
		method: 'get',
		url: '/v1/pt/admin/users',
		params,
	});
};

export const adminUpdateUser = (data: {
	user_id: number;
	class: number;
	custom_title?: string;
	vip_days?: number;
	can_upload?: boolean;
	can_leech?: boolean;
}) => {
	return request({
		method: 'post',
		url: '/v1/pt/admin/user/class',
		data,
	});
};

export interface PromotionRule {
	class: number;
	class_name?: string;
	min_weeks: number;
	min_download_gb: number;
	min_ratio: number;
	demote_ratio: number;
	min_bonus: number;
	award_invites: number;
}

export interface PTAuthorityRules {
	min_class_download: number;
	min_class_upload: number;
	min_class_promotion: number;
	min_class_anonymous: number;
	min_class_view_peers: number;
	retiree_free_upload: boolean;
	retiree_exempt_demote: boolean;
	uploader_karma_multi: number;
	vip_karma_multi: number;
	veteran_karma_multi: number;
	ultimate_karma_multi: number;
	master_karma_multi: number;
	promotion_rules?: PromotionRule[];
}

export const adminGetAuthorityRules = (): Promise<PTAuthorityRules> => {
	return request({
		method: 'get',
		url: '/v1/pt/admin/authority/settings',
	});
};

export const adminSaveAuthorityRules = (data: PTAuthorityRules): Promise<PTAuthorityRules> => {
	return request({
		method: 'post',
		url: '/v1/pt/admin/authority/settings',
		data,
	});
};

export interface PTInviteItem {
	id: number;
	inviter_id: number;
	inviter_name?: string;
	code: string;
	status: number; // 0: 未使用, 1: 已使用, 2: 已过期
	invited_uid: number;
	invited_name?: string;
	created_at: number;
	expire_at: number;
	used_at: number;
}

export interface MyInvitesResp {
	invites: number;
	list: PTInviteItem[];
}

export const getMyInvites = (): Promise<MyInvitesResp> => {
	return request({
		method: 'get',
		url: '/v1/pt/invites',
	});
};

export const generateInvite = (): Promise<PTInviteItem> => {
	return request({
		method: 'post',
		url: '/v1/pt/invite/generate',
	});
};

export const checkInviteCode = (code: string): Promise<{ code: string; inviter_name?: string; expire_at: number }> => {
	return request({
		method: 'get',
		url: '/v1/pt/invite/check',
		params: { code },
	});
};

