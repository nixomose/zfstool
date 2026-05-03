# Default macros if rpmbuild is invoked without --define (e.g. manual rebuild).
%{!?ver:%global ver 0.1.0}
%{!?rel:%global rel 1}

%global debug_package %{nil}

Name:           zfstool
Version:        %{ver}
Release:        %{rel}%{?dist}
Summary:        Read-only ZFS and host inspection tool (agent, web UI, GUI)
License:        MIT
URL:            https://example.com/
Source0:       %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.22
BuildRequires:  systemd-rpm-macros

# ZFS userland is required to use the agent meaningfully; package names vary by distro/repo.
Recommends:     zfs

%description
zfstool runs a small agent that exposes pool and host data over a Unix socket
(HTTP). The web and gui commands provide a browser UI. A systemd unit is
included for the agent (zfstool-agent.service).

%prep
%setup -q -n %{name}-%{version}

%build
export GO111MODULE=on
go build -trimpath -buildmode=pie \
    -ldflags "-s -w -X zfstool/internal/version.Version=%{version}" \
    -o zfstool ./cmd/zfstool

%install
install -D -m0755 zfstool %{buildroot}%{_bindir}/zfstool
install -D -m0644 deploy/zfstool-agent.service %{buildroot}%{_unitdir}/zfstool-agent.service

%files
%doc debian/copyright
%{_bindir}/zfstool
%{_unitdir}/zfstool-agent.service

%post
%systemd_post zfstool-agent.service

%preun
%systemd_preun zfstool-agent.service

%postun
%systemd_postun zfstool-agent.service

%changelog
* Sat May 02 2026 Zfstool Maintainers <zfstool@packages.local> - 0.1.0-1
- Initial RPM packaging
