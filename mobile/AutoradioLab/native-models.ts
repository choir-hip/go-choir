import runtime from "../android-model-runtime.json";

// Shared authority for Android's native-model proof seam. Pages, consent, model
// choice, prompts and audio samples stay here; Kotlin/C++ only execute them.
export type NativeModelKind = "text" | "embedding" | "vad" | "tts";
export type NativeModelFile = { name: string; url: string; sha256: string; bytes: number };
export type NativeModel = {
  id: string;
  kind: NativeModelKind;
  bytes: number;
  embeddingDimension: number;
  supportedVoices: string[];
  files: NativeModelFile[];
};

export const ANDROID_NATIVE_MODELS = runtime.models as NativeModel[];
export const TINY_VOCA_VOICES = ["F2", "M2"] as const;
export const ANDROID_TTS_AVAILABLE = runtime.tts.available;

export function qwen3ChatPrompt(system: string, listener: string): string {
  return `<|im_start|>system\n${system}<|im_end|>\n<|im_start|>user\n${listener}<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n`;
}

export function embeddingDocumentPrompt(title: string, text: string): string {
  return `title: ${title} | text: ${text}`;
}

export function embeddingQueryPrompt(query: string): string {
  return `task: search result | query: ${query}`;
}

export type NativeModelProofRequest = {
  textModelId: string;
  embeddingModelId: string;
  vadModelId: string;
  prompt: string;
  embeddingText: string;
  vadSamples: number[];
  generateTokens: number;
};

// No native defaults: a caller must choose pinned ids and supply the exact
// prompt/text/16 kHz mono samples it wants measured. The native side validates.
export function nativeModelProofRequest(value: NativeModelProofRequest): NativeModelProofRequest {
  return {
    textModelId: value.textModelId,
    embeddingModelId: value.embeddingModelId,
    vadModelId: value.vadModelId,
    prompt: value.prompt,
    embeddingText: value.embeddingText,
    vadSamples: value.vadSamples,
    generateTokens: value.generateTokens,
  };
}
