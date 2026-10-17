'use client';

import { useEffect } from 'react';
import { getAccessToken, getCurrentSession } from '../lib/api';
import { RealtimeClient } from '../lib/realtime';

// A WebSocket is bound to the authenticated identity at ticket issuance.
// Never keep an old user's live stream after another user signs in on the
// same tab, and don't repeatedly open sockets while auth is still restoring.
export function RealtimeBridge() {
  useEffect(() => {
    let mounted = true;
    let revision = 0;
    let activeUser = '';
    let activeClient: RealtimeClient | null = null;

    function stop() {
      activeClient?.stop();
      activeClient = null;
      activeUser = '';
    }

    async function reconcile() {
      const myRevision = ++revision;
      try {
        const session = await getCurrentSession();
        if (!mounted || myRevision !== revision) return;
        if (!session) { stop(); return; }
        if (activeUser === session.user_id && activeClient) return;
        stop();
        activeUser = session.user_id;
        activeClient = new RealtimeClient();
        activeClient.start();
      } catch {
        // Do not destroy a healthy socket on transient auth/API failures.
        // An authenticated retry follows on visibility change or a new
        // auth event; the socket itself handles reconnection on network loss.
      }
    }
    function onAuthChanged() {
      // Explicit logout invalidates access immediately. Do not leave a
      // privileged realtime stream from the previous account connected.
      if (!getAccessToken()) {
        ++revision;
        stop();
        return;
      }
      void reconcile();
    }
    function onForeground() {
      if (document.visibilityState === 'visible') void reconcile();
    }

    void reconcile();
    window.addEventListener('chat:auth-changed', onAuthChanged);
    document.addEventListener('visibilitychange', onForeground);
    return () => {
      mounted = false;
      ++revision;
      window.removeEventListener('chat:auth-changed', onAuthChanged);
      document.removeEventListener('visibilitychange', onForeground);
      stop();
    };
  }, []);
  return null;
}
