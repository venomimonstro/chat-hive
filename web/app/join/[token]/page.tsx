import type { Metadata } from 'next';
import { JoinClient } from './JoinClient';

const API_BASE = process.env.CHAT_INTERNAL_API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type InvitePreview = {
  title: string;
  description?: string;
  members_count: number;
};

async function getInvite(token: string): Promise<InvitePreview | null> {
  try {
    const response = await fetch(`${API_BASE}/api/v1/groups/invites/${encodeURIComponent(token)}/preview`, {
      headers: { Accept: 'application/json' },
      cache: 'no-store'
    });
    if (!response.ok) return null;
    return response.json() as Promise<InvitePreview>;
  } catch {
    return null;
  }
}

function descriptionOf(invite: InvitePreview) {
  const text = (invite.description ?? '').replace(/\s+/g, ' ').trim();
  if (text) return text.length > 180 ? `${text.slice(0, 177)}…` : text;
  return `Присоединяйтесь к группе «${invite.title}» в CHAT · ${invite.members_count} участников`;
}

export async function generateMetadata({ params }: { params: Promise<{ token: string }> }): Promise<Metadata> {
  const { token } = await params;
  const invite = await getInvite(token);
  if (!invite) {
    return {
      title: 'Приглашение в CHAT',
      description: 'Приглашение в группу CHAT',
      robots: { index: false, follow: false }
    };
  }
  const description = descriptionOf(invite);
  return {
    title: `${invite.title} — приглашение в CHAT`,
    description,
    robots: { index: false, follow: false },
    openGraph: {
      type: 'website',
      title: `${invite.title} — CHAT`,
      description
    },
    twitter: {
      card: 'summary',
      title: `${invite.title} — CHAT`,
      description
    }
  };
}

export default function JoinGroupPage() {
  return <JoinClient />;
}
