// Send the same Shell data object as Explorer, without a desktop session.
#include <shlobj.h>
#include <cstdio>
#include <vector>

#define CHECK(call) do { HRESULT hr = (call); if (FAILED(hr)) { \
    std::fprintf(stderr, "%s: HRESULT %08lx\n", #call, (unsigned long)hr); return 1; } } while (0)

int wmain(int argc, wchar_t **argv) {
    if (argc < 4) return 2; // CLSID, common parent, selected absolute paths
    CHECK(CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED));
    CLSID clsid;
    CHECK(CLSIDFromString(argv[1], &clsid));
    PIDLIST_ABSOLUTE parent = nullptr;
    CHECK(SHParseDisplayName(argv[2], nullptr, &parent, 0, nullptr));
    std::vector<PIDLIST_ABSOLUTE> paths(argc - 3);
    std::vector<PCUITEMID_CHILD> children;
    for (int i = 3; i < argc; ++i) {
        auto &path = paths[i - 3];
        CHECK(SHParseDisplayName(argv[i], nullptr, &path, 0, nullptr));
        children.push_back(ILFindLastID(path));
    }
    IDataObject *data = nullptr;
    CHECK(SHCreateDataObject(parent, (UINT)children.size(), children.data(), nullptr, IID_PPV_ARGS(&data)));
    IDropTarget *target = nullptr;
    CHECK(CoCreateInstance(clsid, nullptr, CLSCTX_LOCAL_SERVER, IID_PPV_ARGS(&target)));
    DWORD effect = DROPEFFECT_COPY;
    CHECK(target->Drop(data, 0, POINTL{}, &effect));
    target->Release();
    data->Release();
    for (auto path : paths) CoTaskMemFree(path);
    CoTaskMemFree(parent);
    CoUninitialize();
    return effect == DROPEFFECT_NONE ? 0 : 1;
}
