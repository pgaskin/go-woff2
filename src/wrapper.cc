#include <algorithm>
#include <cstdint>
#include <cstddef>
#include <string>

#include <woff2/decode.h>
#include <woff2/encode.h>
#include <woff2/output.h>

__attribute__((__import_module__("env"),__import_name__(("write"))))
extern bool woff2_write(const void *buf, size_t n);

__attribute__((__import_module__("env"),__import_name__(("write_at"))))
extern bool woff2_write_at(const void *buf, size_t offset, size_t n);

__attribute__((__import_module__("env"),__import_name__(("write_err"))))
extern void woff2_write_err(const void *buf, size_t n);

class WOFF2WrapperOut : public woff2::WOFF2Out {
public:
  WOFF2WrapperOut() {};

  bool Write(const void *buf, size_t n) override {
    bool ok = woff2_write(buf, n);
    if (ok) {
        offset_ += n;
        size_ += n;
    }
    return ok;
  };

  bool Write(const void *buf, size_t offset, size_t n) override {
    if (offset > size_ || n > size_ - offset) {
        return false;
    }
    bool ok = woff2_write_at(buf, offset, n);
    if (ok) {
        offset_ = std::max(offset_, offset + n);
    }
    return ok;
  };

  size_t Size() override {
    return offset_;
  }

private:
  size_t size_ = 0;
  size_t offset_ = 0;
};

__attribute__((visibility("default")))
extern "C" bool woff2_decode(const uint8_t *data, size_t length) {
    WOFF2WrapperOut out;
    return woff2::ConvertWOFF2ToTTF(data, length, &out);
}

__attribute__((visibility("default")))
extern "C" size_t woff2_decode_size(const uint8_t *data, size_t length) {
    return woff2::ComputeWOFF2FinalSize(data, length);
}

struct woff2_encode_result {
    size_t length;
    bool ok;
};

__attribute__((visibility("default")))
extern "C" struct woff2_encode_result woff2_encode(const uint8_t *data, size_t length, uint8_t *result, size_t result_length, const char *extended_metadata, size_t extended_metadata_length, int brotli_quality, bool allow_transforms) {
    woff2::WOFF2Params params;
    if (extended_metadata_length > 0) {
        params.extended_metadata = std::string(extended_metadata, extended_metadata_length);
    }
    if (brotli_quality != 0) {
        params.brotli_quality = brotli_quality;
    }
    params.allow_transforms = allow_transforms;
    bool ok = woff2::ConvertTTFToWOFF2(data, length, result, &result_length, params);
    return woff2_encode_result{
        .length = result_length,
        .ok = ok,
    };
}

__attribute__((visibility("default")))
extern "C" size_t woff2_encode_size_max(const uint8_t *data, size_t length, const char *extended_metadata, size_t extended_metadata_length) {
    std::string meta;
    if (extended_metadata_length > 0) {
        meta = std::string(extended_metadata, extended_metadata_length);
    }
    return woff2::MaxWOFF2CompressedSize(data, length, meta);
}

// no-op fd_write (referenced by stdio and abort)
extern "C" int32_t __imported_wasi_snapshot_preview1_fd_write(int32_t fd, int32_t iovs, int32_t iovs_len, int32_t nwritten) {
    struct iovec_t { uint32_t buf; uint32_t len; };
    const iovec_t *v = (const iovec_t *)(uintptr_t)iovs;
    uint32_t total = 0;
    for (int32_t i = 0; i < iovs_len; i++) {
        if (fd == 2 && v[i].len > 0) {
            // forward stderr to catch FONT_COMPRESSION_DEBUG/FONT_COMPRESSION_BIN messages
            woff2_write_err((const void *)(uintptr_t)v[i].buf, v[i].len);
        }
        total += v[i].len;
    }
    *(uint32_t *)(uintptr_t)nwritten = total;
    return 0;
}

// no-op fd_seek (referenced by stdio and abort)
extern "C" int32_t __imported_wasi_snapshot_preview1_fd_seek(int32_t, int64_t, int32_t, int32_t newoffset) {
    *(int64_t *)(uintptr_t)newoffset = 0;
    return 0;
}

// no-op fd_close (referenced by stdio and abort)
extern "C" int32_t __imported_wasi_snapshot_preview1_fd_close(int32_t) {
    return 0;
}

// no-op proc_exit (referenced by abort)
__attribute__((noreturn)) extern "C" void __imported_wasi_snapshot_preview1_proc_exit(int32_t) {
    __builtin_trap();
}
