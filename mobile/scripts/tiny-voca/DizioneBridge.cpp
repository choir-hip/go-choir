// C ABI around the unchanged, standalone (Apache-2.0) Dizione C++ port.
// Native only because this is Tiny Voca's text frontend and runs beside the
// Core ML voice; the shared Exact code owns the page, the choice and the queue.
//
// Output format of ar_dizione_phonemize, a flat int32 stream owned by the
// caller until ar_dizione_free:
//   U, n0, ids0[n0], stress0[n0], n1, ids1[n1], stress1[n1], ...
// One entry per utterance: a sentence, or a sentence cut at a word boundary
// when it would exceed the runner's token bucket.
#include "dizione.h"

#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <exception>
#include <string>
#include <vector>

namespace {
thread_local std::string gError;
constexpr int kEosLike = 0;   // unused; kept explicit for the reader

bool is_sentence_mark(const std::string &s) {
    return s == "." || s == "?" || s == "!" || s == "\xE2\x80\xA6";   // … as UTF-8
}
}  // namespace

extern "C" {

// Opens the phonemizer data in `directory` (lexicon.tsv, align.json, g2p.json,
// g2p.npz, homographs.json). nullptr on failure; see ar_dizione_error.
void *ar_dizione_create(const char *directory) {
    try {
        return new dizione::Phonemizer(dizione::load_directory(directory ? directory : ""));
    } catch (const std::exception &e) {
        gError = e.what();
        return nullptr;
    }
}

void ar_dizione_destroy(void *phonemizer) {
    delete static_cast<dizione::Phonemizer *>(phonemizer);
}

const char *ar_dizione_error(void) {
    return gError.c_str();
}

// Phonemizes UTF-8 `text` into utterances of at most `max_tokens` tokens each.
// On success sets *out_data and *out_count (int32 elements) and returns 0.
// The caller frees *out_data with ar_dizione_free. Returns 1 on a refusal
// (nothing to pronounce, bad text) and 2 on allocation failure.
int ar_dizione_phonemize(void *phonemizer, const char *text, int max_tokens,
                         int32_t **out_data, int32_t *out_count) {
    *out_data = nullptr;
    *out_count = 0;
    if (!phonemizer || !text || max_tokens < 4) {
        gError = "bad arguments";
        return 1;
    }
    std::vector<std::vector<int32_t>> ids, stress;
    try {
        auto result = static_cast<dizione::Phonemizer *>(phonemizer)->phonemize(text);
        std::vector<int32_t> curIds, curStress;
        for (size_t i = 0; i < result.ids.size(); ++i) {
            const bool mark = i < result.symbols.size() && is_sentence_mark(result.symbols[i]);
            curIds.push_back(result.ids[i]);
            curStress.push_back(result.stress[i]);
            const bool full = static_cast<int>(curIds.size()) >= max_tokens;
            if (mark || full) {
                ids.push_back(std::move(curIds));
                stress.push_back(std::move(curStress));
                curIds.clear();
                curStress.clear();
            }
        }
        if (!curIds.empty()) {
            ids.push_back(std::move(curIds));
            stress.push_back(std::move(curStress));
        }
    } catch (const std::exception &e) {
        gError = e.what();
        return 1;
    }
    int32_t total = 1;
    for (const auto &u : ids) total += 1 + static_cast<int32_t>(u.size()) * 2;
    auto *out = static_cast<int32_t *>(std::malloc(sizeof(int32_t) * static_cast<size_t>(total)));
    if (!out) {
        gError = "out of memory";
        return 2;
    }
    int32_t *p = out;
    *p++ = static_cast<int32_t>(ids.size());
    for (size_t u = 0; u < ids.size(); ++u) {
        *p++ = static_cast<int32_t>(ids[u].size());
        std::memcpy(p, ids[u].data(), sizeof(int32_t) * ids[u].size());
        p += ids[u].size();
        std::memcpy(p, stress[u].data(), sizeof(int32_t) * stress[u].size());
        p += stress[u].size();
    }
    *out_data = out;
    *out_count = total;
    return 0;
}

void ar_dizione_free(void *data) {
    std::free(data);
}

}  // extern "C"
