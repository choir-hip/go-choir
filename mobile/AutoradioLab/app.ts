import type { Answer, Sources } from './app.contract.d.ts';

// The microphone and on-device speech recognition; the screen may be locked while the radio plays.
export const grants = `device.microphone purpose.microphone
device.microphone-background purpose.recordingInBackground
device.speech-recognition purpose.speech`;

const sources: Sources = {};
export const answer: Answer = (source, args, store, storage) => sources[source](args, store, storage);
