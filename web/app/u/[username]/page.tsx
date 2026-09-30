import type { Metadata } from 'next';
import { ProfileClient } from './ProfileClient';

const API_BASE = process.env.CHAT_INTERNAL_API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type PublicProfile = {
  username: string;
  display_name: string;
  bio: string;
  followers_count: number;
};

async function getProfile(username: string): Promise<PublicProfile | null> {
  try {
    const response = await fetch(`${API_BASE}/api/v1/profiles/${encodeURIComponent(username)}`, {
      headers: { Accept: 'application/json' }, cache: 'no-store'
    });
    if (!response.ok) return null;
    return response.json() as Promise<PublicProfile>;
  } catch {
    return null;
  }
}

function summary(value: string, fallback: string) {
  const compact = value.replace(/\s+/g, ' ').trim();
  if (!compact) return fallback;
  return compact.length > 180 ? `${compact.slice(0, 177)}…` : compact;
}

export async function generateMetadata({ params }: { params: Promise<{ username: string }> }): Promise<Metadata> {
  const { username } = await params;
  const profile = await getProfile(username);
  if (!profile) return { title: 'Профиль — CHAT', robots: { index: false, follow: false } };
  const name = profile.display_name || `@${profile.username}`;
  const description = summary(profile.bio, `${name} в CHAT · ${profile.followers_count} подписчиков`);
  return {
    title: `${name} (@${profile.username}) — CHAT`,
    description,
    openGraph: { type: 'profile', title: `${name} — CHAT`, description },
    twitter: { card: 'summary', title: `${name} — CHAT`, description }
  };
}

export default async function PublicProfilePage({ params }: { params: Promise<{ username: string }> }) {
  const { username } = await params;
  return <ProfileClient username={username} />;
}
