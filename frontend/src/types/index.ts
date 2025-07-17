export interface FileMetadata {
  id: string;
  filename: string;
  size: number;
  compressed_size: number;
  mime_type: string;
  compression: string;
  upload_time: string;
  expires_at: string;
  has_download_password: boolean;
}

export interface UploadResult {
  success: boolean;
  filename: string;
  fileId?: string;
  metadata?: FileMetadata;
  error?: string;
  delete_password?: string;
}

export interface ZipFile {
  name: string;
  size: number;
  compressed: number;
  modified: string;
  is_dir: boolean;
  method: number;
}

export interface ZipContents {
  filename: string;
  files: ZipFile[];
  total: number;
}