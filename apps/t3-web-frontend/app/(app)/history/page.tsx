'use client';

import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { keys } from '@/lib/queries';
import { useStation } from '@/lib/station';
import { TrackTable } from '@/components/tracks/TrackTable';
import { PageHeader } from '@/components/ui/Layout';
import { TabPanel, Tabs } from '@/components/ui/Tabs';

type Tab = 'history' | 'liked' | 'banned';

export default function HistoryPage() {
  const [tab, setTab] = useState<Tab>('history');
  const { stations } = useStation();
  const { data: history } = useQuery(keys.history, () => api.listHistory());
  const { data: liked } = useQuery(keys.feedback('like'), () => api.listFeedback('like'));
  const { data: banned } = useQuery(keys.feedback('ban'), () => api.listFeedback('ban'));

  const historyRows = useMemo(() => {
    const names = new Map(stations.map((s) => [s.id, s.name]));
    return (history ?? []).map((h) => ({
      id: h.track_id,
      title: h.title,
      artist: h.artist,
      album: h.album,
      played_at: h.played_at,
      station: h.station_id ? names.get(h.station_id) : undefined,
    }));
  }, [history, stations]);

  return (
    <>
      <PageHeader title="History & feedback" />
      <Tabs
        label="History sections"
        idPrefix="history"
        value={tab}
        onChange={setTab}
        tabs={[
          { id: 'history', label: 'History', count: history?.length },
          { id: 'liked', label: 'Liked', count: liked?.length },
          { id: 'banned', label: 'Banned', count: banned?.length },
        ]}
      />
      <TabPanel idPrefix="history" id={tab}>
        {tab === 'history' ? (
          <TrackTable
            label="Listening history"
            rows={historyRows}
            columns={['title', 'album', 'station', 'played_at']}
            sortable
            empty="No history yet."
          />
        ) : (
          <TrackTable
            label={tab === 'liked' ? 'Liked tracks' : 'Banned tracks'}
            rows={(tab === 'liked' ? liked : banned) ?? []}
            columns={['title', 'album', 'duration', 'sample_rate']}
            sortable
            empty={`No ${tab} tracks yet.`}
          />
        )}
      </TabPanel>
    </>
  );
}
