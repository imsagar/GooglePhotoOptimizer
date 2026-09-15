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
  base_url?: string;
}

export interface Job {
  id: number;
  video_id: string;
  filename?: string;
  status: "queued" | "downloading" | "encoding" | "ready" | "uploading" | "uploaded" | "failed" | "cancelled";
  run_date: string;
  original_size: number;
  optimized_size: number;
  savings_pct: number;
  codec: string;
  crf: number;
  preset: string;
  drive_file_id?: string;
  error?: string;
  progress: number;
  delete_original: boolean;
  downloaded_at?: string;
  optimized_at?: string;
  uploaded_at?: string;
  created_at?: string;
}

export interface Paginated<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}
