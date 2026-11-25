/* The C interface to Tiny Voca's text frontend (Dizione). Pure C, so Swift can
 * import it: the C++ port behind it is in DizioneBridge.cpp. The phonemizer
 * data lives in a directory the app bundles (TinyVoca/phonemizer). */
#ifndef TINY_VOCA_DIZIONE_ABI_H
#define TINY_VOCA_DIZIONE_ABI_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Opens the data in `directory`. nullptr on failure; see ar_dizione_error. */
void *ar_dizione_create(const char *directory);

void ar_dizione_destroy(void *phonemizer);

/* The last error, as UTF-8; "" when there is none. */
const char *ar_dizione_error(void);

/* Phonemizes UTF-8 `text` into utterances of at most `max_tokens` tokens,
 * each cut at a sentence mark. On success *out_data holds a flat int32 stream
 * owned by the caller until ar_dizione_free, in this layout:
 *   U, n0, ids0[n0], stress0[n0], n1, ids1[n1], stress1[n1], ...
 * Returns 0 on success, 1 on a refusal (nothing to pronounce, bad text) and
 * 2 when memory ran out. */
int ar_dizione_phonemize(void *phonemizer, const char *text, int max_tokens,
                         int32_t **out_data, int32_t *out_count);

void ar_dizione_free(void *data);

#ifdef __cplusplus
}
#endif

#endif
