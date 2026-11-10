import type { Answer, Sources } from './app.contract.d.ts';

export const grants = `device.microphone purpose.microphone\ndevice.microphone-background purpose.recordingInBackground`;

const sources: Sources = {};
export const answer: Answer = (source, args, store, storage) => sources[source](args, store, storage);
