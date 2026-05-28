export type Team = {
  id: string;
  name: string;
  description?: string;
  currency: string;
  is_public: boolean;
  owner_id: string;
  created_at: string;
};

export type Member = {
  id: string;
  user_id: string;
  display_name: string;
  identity_type: string;
  role: string;
  status: string;
  joined_at?: string;
};

export type PickedMember = {
  id: string;
  name: string;
  isAnonymous?: boolean;
};
