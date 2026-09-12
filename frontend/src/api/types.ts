export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url: string;
}

export interface Video {
  id: string;
  filename: string;
  mime_type: string;
  size_bytes: number;
  duration_ms: number;
  width: number;
  height: number;
  creation_time: string;
  album_id?: string;
  album_title?: string;
}

export interface Job {
  id: number;
  video_id: string;
  status: "queued" | "downloading" | "encoding" | "ready" | "uploading" | "uploaded" | "failed" | "cancelled";
  run_date: string;
  original_size: number;
  optimized_size: number;
  savings_pct: number;
  codec: string;
  crf: number;
  preset: string;
  error?: string;
  progress: number;
  delete_original: boolean;
}

export interface Paginated<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}
