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
