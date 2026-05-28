export type Notification = {
  id: string;
  type: string;
  entity_type: string;
  entity_id: string;
  payload?: Record<string, unknown>;
  is_read: boolean;
  read_at?: string;
  created_at: string;
};

export type NotificationPage = {
  items: Notification[];
  next_cursor?: string;
  has_more: boolean;
};
