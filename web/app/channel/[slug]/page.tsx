import type { Metadata } from 'next';
import { ChannelClient } from './ChannelClient';

const API_BASE = process.env.CHAT_INTERNAL_API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type PublicChannel = { slug: string; title: string; description: string; subscribers_count: number };

async function getPublicChannel(slug: string): Promise<PublicChannel | null> {
  try {
    const response = await fetch(`${API_BASE}/api/v1/channels/${encodeURIComponent(slug)}`, {
      headers: { Accept: 'application/json' }, cache: 'no-store'
    });
    if (!response.ok) return null;
    return response.json() as Promise<PublicChannel>;
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
  const channel = await getPublicChannel(slug);
  if (!channel) return { title: 'Канал — CHAT', robots: { index: false, follow: false } };
  const description = summary(channel.description, `${channel.title} · ${channel.subscribers_count} подписчиков в CHAT`);
  return {
    title: `${channel.title} (@${channel.slug}) — CHAT`,
    description,
    openGraph: { type: 'website', title: `${channel.title} — CHAT`, description },
    twitter: { card: 'summary', title: `${channel.title} — CHAT`, description }
  };
}

export default async function ChannelPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  return <ChannelClient slug={slug} />;
}
