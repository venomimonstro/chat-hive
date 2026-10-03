import type { Metadata } from 'next';
import { GrowthView } from '../../../components/GrowthView';
import { PostClient } from './PostClient';

const API_BASE = process.env.CHAT_INTERNAL_API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? 'http://localhost:8080';

type PublicPost = {
  author_username: string;
  author_name: string;
  body: string;
  kind: string;
  media?: Array<{ url: string }>;
};

async function getPublicPost(id: string): Promise<PublicPost | null> {
  try {
    const response = await fetch(`${API_BASE}/api/v1/posts/${encodeURIComponent(id)}`, {
      headers: { Accept: 'application/json' }, cache: 'no-store'
    });
    if (!response.ok) return null;
    return response.json() as Promise<PublicPost>;
  } catch {
    return null;
  }
}

function summary(value: string, fallback: string) {
  const compact = value.replace(/\s+/g, ' ').trim();
  if (!compact) return fallback;
  return compact.length > 180 ? `${compact.slice(0, 177)}…` : compact;
}

export async function generateMetadata({ params }: { params: Promise<{ id: string }> }): Promise<Metadata> {
  const { id } = await params;
  const post = await getPublicPost(id);
  if (!post) return { title: 'Публикация — CHAT', robots: { index: false, follow: false } };
  const author = post.author_name || `@${post.author_username}`;
  const description = summary(post.body, `Публикация ${author} в CHAT`);
  const image = post.media?.[0]?.url;
  const absoluteImage = image && API_BASE.startsWith('https://') ? `${API_BASE}${image}` : undefined;
  return {
    title: `${author} — CHAT`,
    description,
    openGraph: {
      type: 'article',
      title: `${author} — CHAT`,
      description,
      images: absoluteImage ? [{ url: absoluteImage }] : undefined
    },
    twitter: { card: absoluteImage ? 'summary_large_image' : 'summary', title: `${author} — CHAT`, description, images: absoluteImage ? [absoluteImage] : undefined }
  };
}

export default async function PostPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <><GrowthView objectType="post" objectId={id} /><PostClient postId={id} /></>;
}
