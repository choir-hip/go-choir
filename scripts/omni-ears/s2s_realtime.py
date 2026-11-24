#!/usr/bin/env python3
"""Speech in, speech out through llama-server's /v1/realtime WebSocket (OpenAI-realtime-style events; b12259 `llama serve`).
s2s_realtime.py <port> <in.wav 16 kHz mono | in.txt (text in, e.g. instructions 'Perform TTS in American English using a female voice.')> <out.wav> [instructions]  -> prints event types, time to first audio delta, total, text.
Output audio is 24 kHz pcm16 (the model card). Protocol names are the ones in the server's binary; unknown ones are printed, not guessed."""
import asyncio, base64, json, os, sys, time, wave
import websockets
port, inp, outp = sys.argv[1], sys.argv[2], sys.argv[3]
instr = sys.argv[4] if len(sys.argv) > 4 else 'Respond with interleaved text and audio.'
async def main():
    textmode = inp.endswith('.txt'); pcm = b'' if textmode else wave.open(inp).readframes(10**9)
    async with websockets.connect(f'ws://localhost:{port}/v1/realtime', max_size=None) as ws:
        async def ev(): return json.loads(await asyncio.wait_for(ws.recv(), 120))
        first = await ev(); print('first', first.get('type'))
        await ws.send(json.dumps({'type': 'session.update', 'session': {'type': 'realtime', 'instructions': instr, 'output_modalities': json.loads(os.environ.get('OMNI_MODS', '["audio"]')), 'audio': {'input': {'format': {'type': 'audio/pcm', 'rate': 16000}}}}}))
        t0 = time.time()
        step = 3200 * 2
        if textmode:
            await ws.send(json.dumps({'type': 'conversation.item.create', 'item': {'type': 'message', 'role': 'user', 'content': [{'type': 'input_text', 'text': open(inp).read().strip()}]}}))
        else:
            for i in range(0, len(pcm), step):
                await ws.send(json.dumps({'type': 'input_audio_buffer.append', 'audio': base64.b64encode(pcm[i:i + step]).decode()}))
            await ws.send(json.dumps({'type': 'input_audio_buffer.commit'}))
        await ws.send(json.dumps({'type': 'response.create'}))
        t1 = time.time(); audio = b''; tfa = None; text = ''; seen = []
        while True:
            e = await ev(); ty = e.get('type'); 
            if ty not in seen: seen.append(ty)
            if ty == 'response.output_audio.delta':
                if tfa is None: tfa = time.time() - t1
                audio += base64.b64decode(e['delta'])
            elif ty in ('response.output_text.delta', 'response.output_audio_transcript.delta'): text += e.get('delta', '')
            elif ty == 'error': print('ERROR', e); break
            elif ty in ('response.done', 'response.completed'): break
        tot = time.time() - t1
    w = wave.open(outp, 'wb'); w.setnchannels(1); w.setsampwidth(2); w.setframerate(24000); w.writeframes(audio); w.close()
    print(json.dumps({'events': seen, 'first_audio_s_after_response_create': tfa, 'total_s': tot, 'audio_s': len(audio) / 48000, 'text': text}))
asyncio.run(main())
