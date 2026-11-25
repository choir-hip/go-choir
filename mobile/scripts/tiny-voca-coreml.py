"""Export the pinned port; coremlcompiler separately checks each shipping target."""
from pathlib import Path
from functools import partial
import sys

import coremltools as ct
import torch
from tiny_voca.infer import load_model
from tiny_voca_ane import runtime

checkpoint, out = map(Path, sys.argv[1:])
torch.set_num_threads(4)
# The verified weights are loaded with strict state keys, format version and
# weights_only=True. Conversion does not phonemize any text. JS data comes from
# the same weights release; installing the Python frontend is unnecessary.
model = load_model(checkpoint, "cpu", check_frontend=False)
# Skip GPU warm-up during conversion: on macOS 27 MPSGraph aborts during
# MPSMemrefAllocFusion on the dynamic vocoder. Xcode compiles the packages.
ct.convert = partial(ct.convert, skip_model_load=True)
runtime.compile_package = lambda package, **_: package.with_suffix(".mlmodelc")
runtime.export_models(model, checkpoint, out, log=lambda text: print(text.replace("converted and compiled", "exported"), flush=True))
