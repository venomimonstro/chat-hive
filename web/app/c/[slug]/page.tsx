import type { Metadata } from 'next';
import { CommunityClient } from './CommunityClient';

const API_BASE = process.env.CHAT_INTERNAL_API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type PublicCommunity = { slug: string; title: string; description: string; members_count: number };

async function getPublicCommunity(slug: string): Promise<PublicCommunity | null> {
  try {
    const response = await fetch(`${API_BASE}/api/v1/communities/${encodeURIComponent(slug)}`, {
      headers: { Accept: 'application/json' }, cache: 'no-store'
    });
    if (!response.ok) return null;
    return response.json() as Promise<PublicCommunity>;
  } catch {
    return null;
  }
}

function summary(value: string, fallback: string) {
  const compact = value.replace(/\s+/g, ' ').trim();
  if (!compact) return fallback;
  return compact.length > 180 ? `${compact.slice(0, 177)}…` : compact;
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const community = await getPublicCommunity(slug);
  if (!community) return { title: 'Сообщество — CHAT', robots: { index: false, follow: false } };
  const description = summary(community.description, `${community.title} · ${community.members_count} участников в CHAT`);
  return {
    title: `${community.title} (@${community.slug}) — CHAT`,
    description,
    openGraph: { type: 'website', title: `${community.title} — CHAT`, description },
    twitter: { card: 'summary', title: `${community.title} — CHAT`, description }
  };
}

export default async function CommunityPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  return <CommunityClient slug={slug} />;
}
