MAKEFLAGS += -r

WASM2GO_VERSION := 2bb7e06cd592bdfd4b6580818a8060aaa6e046ed
WOFF2_VERSION   := $(shell cut -c1-10 woff2/COMMIT)
BROTLI_VERSION  := $(shell cut -c1-10 brotli/COMMIT)

CC       := wasm32-wasip1-clang
CXX      := wasm32-wasip1-clang++
GO       := $(if $(GOROOT),$(GOROOT)/bin/,)go
WASM_OPT := wasm-opt
WASM2GO  := $(GO) run github.com/ncruces/wasm2go@$(WASM2GO_VERSION)

OUTDIR := ../internal/

LDFLAGS := \
	-mexec-model=reactor \
	-Wl,--export=woff2_decode \
	-Wl,--export=woff2_decode_size \
	-Wl,--export=woff2_encode \
	-Wl,--export=woff2_encode_size_max \
	-Wl,--export=malloc \
	-Wl,--export=free \
	-Wl,--no-entry \
	-Wl,--gc-sections

OPTFLAGS := \
	-g \
	-O3 \
	--enable-bulk-memory \
	--enable-mutable-globals \
	--enable-sign-ext \
	--enable-nontrapping-float-to-int \
	--enable-multivalue

CFLAGS := \
	-g \
	-ffile-prefix-map== \
	-ffile-prefix-map=woff2=github.com/google/woff2@$(WOFF2_VERSION) \
	-ffile-prefix-map=brotli=github.com/google/brotli@$(BROTLI_VERSION) \
	-ffile-compilation-dir=. \
	-O3 \
	-fno-exceptions \
	-fno-rtti \
	-fno-threadsafe-statics \
	-fvisibility=hidden \
	-DNDEBUG \
	-Wall \
	-Wno-unused-function \
	-Ibrotli/c/include

CXXFLAGS := \
	$(CFLAGS) \
	-std=c++17 \
	-Iwoff2/include

# wasi-sdk's no-exceptions libc++ pulls <locale> (via <vector>) whose num.h uses
# errno without including <cerrno>, so force-include it
CXXFLAGS += -include cerrno

WOFF2_FLAGS := \
	-Iwoff2/src

BROTLI_FLAGS := \
	-Ibrotli/c/include

WRAPPER_FLAGS := \
	-mmultivalue \
	-Xclang -target-abi \
	-Xclang experimental-mv

# google/woff2 (encode, decode).
WOFF2_SRCS := \
	woff2_enc.cc \
	woff2_dec.cc \
	woff2_common.cc \
	woff2_out.cc \
	font.cc \
	glyph.cc \
	normalize.cc \
	transform.cc \
	table_tags.cc \
	variable_length.cc

# brotli (common, decoder, encoder)
BROTLI_SRCS := \
	common/dictionary.c \
	common/transform.c \
	dec/bit_reader.c \
	dec/decode.c \
	dec/huffman.c \
	dec/state.c \
	enc/backward_references.c \
	enc/backward_references_hq.c \
	enc/bit_cost.c \
	enc/block_splitter.c \
	enc/brotli_bit_stream.c \
	enc/cluster.c \
	enc/compress_fragment.c \
	enc/compress_fragment_two_pass.c \
	enc/dictionary_hash.c \
	enc/encode.c \
	enc/encoder_dict.c \
	enc/entropy_encode.c \
	enc/histogram.c \
	enc/literal_cost.c \
	enc/memory.c \
	enc/metablock.c \
	enc/static_dict.c \
	enc/utf8_util.c

WRAPPER_SRCS := \
	wrapper.cc

OBJS := \
	$(addprefix obj/woff2/,$(addsuffix .o,$(WOFF2_SRCS))) \
	$(addprefix obj/brotli/,$(addsuffix .o,$(BROTLI_SRCS))) \
	$(addprefix obj/,$(addsuffix .o,$(WRAPPER_SRCS)))

.PHONY: all clean distclean download
all: $(OUTDIR)woff2.go

clean:
	rm -rf obj *.wasm

distclean: clean
	rm -rf woff2 brotli

download:
	CGO_ENABLED=0 $(GO) run ./download.go

$(OUTDIR)woff2.go: woff2.opt.wasm
	CGO_ENABLED=0 $(WASM2GO) -embed -dwarfline -pkg internal -o $@ $<

woff2.opt.wasm: woff2.wasm
	$(WASM_OPT) $(OPTFLAGS) -o $@ $<

woff2.wasm: $(OBJS)
	$(CXX) $(LDFLAGS) -o $@ $(OBJS)

obj/%.o: %
	@mkdir -p $(dir $@)
	$(CXX) $(CXXFLAGS) $(WRAPPER_FLAGS) -c -o $@ $<

obj/woff2/%.o: woff2/src/%
	@mkdir -p $(dir $@)
	$(CXX) $(CXXFLAGS) $(WOFF2_FLAGS) -c -o $@ $<

obj/brotli/%.o: brotli/c/%
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) $(BROTLI_FLAGS) -c -o $@ $<
