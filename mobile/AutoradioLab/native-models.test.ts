import { describe, expect, test } from "bun:test";
import runtime from "../android-model-runtime.json";
import {
  embeddingDocumentPrompt,
  embeddingQueryPrompt,
  nativeModelProofRequest,
  qwen3ChatPrompt,
  TINY_VOCA_VOICES,
} from "./native-models";

// Failure list before the implementation:
//  N1 Qwen's shared prompt differs from AutoradioKit's ChatML layout or loses the empty think block.
//  N2 EmbeddingGemma document/query prompts differ from the pinned card's forms.
//  N3 a model id, URL, byte count or digest is missing/changes independently of the native execution mirror.
//  N4 a proof request invents model choices, a prompt, VAD samples or a token count.
//  N5 Tiny Voca is reported as synthesized before an Android implementation exists.

describe("Android native model shared contract", () => {
  test("Qwen3 uses ChatML with thinking off", () => {
    expect(qwen3ChatPrompt("System", "Listener")).toBe(
      "<|im_start|>system\nSystem<|im_end|>\n<|im_start|>user\nListener<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n",
    );
  });

  test("EmbeddingGemma keeps the card's titled document and query forms", () => {
    expect(embeddingDocumentPrompt("Harbor", "The council voted.")).toBe(
      "title: Harbor | text: The council voted.",
    );
    expect(embeddingQueryPrompt("Why did the council vote?")).toBe(
      "task: search result | query: Why did the council vote?",
    );
  });

  test("the shared pins are complete and use the existing model ids", () => {
    expect(runtime.models.map((model) => model.id)).toEqual([
      "qwen3-0.6b-q4_k_m",
      "qwen3-0.6b-q8_0",
      "embeddinggemma-2-q8_0",
      "silero-vad-v6.2.1",
      "tiny-voca",
    ]);
    for (const model of runtime.models) {
      expect(model.bytes).toBeGreaterThan(0);
      for (const file of model.files) {
        expect(file.url.startsWith("https://")).toBe(true);
        expect(file.sha256).toMatch(/^[0-9a-f]{64}$/);
        expect(file.name).not.toContain("/");
      }
    }
    expect(runtime.models.find((model) => model.id === "embeddinggemma-2-q8_0")?.embeddingDimension).toBe(768);
    expect(TINY_VOCA_VOICES).toEqual(["F2", "M2"]);
    expect(runtime.tts).toEqual({ available: false, reason: "Tiny Voca synthesis is not implemented on Android." });
  });

  test("a proof request carries shared choices and inputs without native defaults", () => {
    expect(nativeModelProofRequest({
      textModelId: "qwen3-0.6b-q4_k_m",
      embeddingModelId: "embeddinggemma-2-q8_0",
      vadModelId: "silero-vad-v6.2.1",
      prompt: "<|im_start|>user\nHello<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n",
      embeddingText: "task: search result | query: harbor",
      vadSamples: [0, 0, 0],
      generateTokens: 8,
    })).toEqual({
      textModelId: "qwen3-0.6b-q4_k_m",
      embeddingModelId: "embeddinggemma-2-q8_0",
      vadModelId: "silero-vad-v6.2.1",
      prompt: "<|im_start|>user\nHello<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n",
      embeddingText: "task: search result | query: harbor",
      vadSamples: [0, 0, 0],
      generateTokens: 8,
    });
  });
});
