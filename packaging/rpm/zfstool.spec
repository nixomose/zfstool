# Default macros if rpmbuild is invoked without --define (e.g. manual rebuild).
%{!?ver:%global ver 0.1.0}
%{!?rel:%global rel 1}

%global debug_package %{nil}

Name:           zfstool
Version:        %{ver}
Release:        %{rel}%{?dist}
Summary:        Read-only ZFS and host inspection tool (API server, web UI, GUI)
License:        MIT
URL:            https://github.com/nixomose/zfstool
Source0:       %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.22
BuildRequires:  gcc
BuildRequires:  gcc-c++
BuildRequires:  pkgconfig(gtk+-3.0)
BuildRequires:  pkgconfig(webkit2gtk-4.1)
BuildRequires:  systemd-rpm-macros

# ZFS userland is required to use the API server meaningfully; package names vary by distro/repo.
Recommends:     zfs

%description
zfstool runs a small API server (zfstool server) that exposes pool and host
data over a Unix socket (HTTP). The default command opens a GTK UI when built
with CGO and GTK; use zfstool web for the browser front-end (after starting
the API server). A systemd unit is included (zfstool-agent.service).

%prep
%setup -q -n %{name}-%{version}

%build
export GO111MODULE=on
export CGO_ENABLED=1
export GOFLAGS=
go build -trimpath -buildmode=pie \
    -ldflags "-s -w -X github.com/nixomose/zfstool/internal/version.Version=%{version}" \
    -o zfstool ./cmd/zfstool

%install
install -D -m0755 zfstool %{buildroot}%{_bindir}/zfstool
install -D -m0644 deploy/zfstool-agent.service %{buildroot}%{_unitdir}/zfstool-agent.service
install -D -m0644 deploy/zfstool.desktop %{buildroot}%{_datadir}/applications/zfstool.desktop
for sz in 16x16 24x24 32x32 48x48 64x64 128x128 256x256 512x512; do
  install -D -m0644 deploy/icons/hicolor/${sz}/apps/zfstool.png \
    %{buildroot}%{_datadir}/icons/hicolor/${sz}/apps/zfstool.png
done
install -D -m0644 deploy/icons/hicolor/scalable/apps/zfstool.svg \
  %{buildroot}%{_datadir}/icons/hicolor/scalable/apps/zfstool.svg

%files
%doc debian/copyright
%{_bindir}/zfstool
%{_unitdir}/zfstool-agent.service
%{_datadir}/applications/zfstool.desktop
%{_datadir}/icons/hicolor/*/apps/zfstool.png
%{_datadir}/icons/hicolor/scalable/apps/zfstool.svg

%post
%systemd_post zfstool-agent.service

%preun
%systemd_preun zfstool-agent.service

%postun
%systemd_postun zfstool-agent.service

%changelog
* Sat May 02 2026 Zfstool Maintainers <zfstool@packages.local> - 0.1.0-1
- Initial RPM packaging
