%undefine _disable_source_fetch

Name:    picoflux
Version: %{_miniflux_version}
Release: 1.0
Summary: Minimalist and opinionated feed reader
URL: https://miniflux.app/
License: ASL 2.0
Source0: picoflux
Source1: picoflux.service
Source2: picoflux.conf
Source3: picoflux.1
Source4: LICENSE
BuildRoot: %{_topdir}/BUILD/%{name}-%{version}-%{release}
BuildArch: x86_64
Requires(pre): shadow-utils

%{?systemd_ordering}

AutoReqProv: no

%define __strip /bin/true
%define __os_install_post %{nil}

%description
%{summary}

%install
mkdir -p %{buildroot}%{_bindir}
install -p -m 755 %{SOURCE0} %{buildroot}%{_bindir}/picoflux
install -D -m 644 %{SOURCE1} %{buildroot}%{_unitdir}/picoflux.service
install -D -m 600 %{SOURCE2} %{buildroot}%{_sysconfdir}/picoflux.conf
install -D -m 644 %{SOURCE3} %{buildroot}%{_mandir}/man1/picoflux.1
install -D -m 644 %{SOURCE4} %{buildroot}%{_docdir}/picoflux/LICENSE

%files
%defattr(755,root,root)
%{_bindir}/picoflux
%{_docdir}/picoflux
%defattr(644,root,root)
%{_unitdir}/picoflux.service
%{_mandir}/man1/picoflux.1*
%{_docdir}/picoflux/*
%defattr(600,root,root)
%config(noreplace) %{_sysconfdir}/picoflux.conf

%pre
getent group picoflux >/dev/null || groupadd -r picoflux
getent passwd picoflux >/dev/null || \
    useradd -r -g picoflux -d /dev/null -s /sbin/nologin \
    -c "Miniflux Daemon" picoflux
exit 0

%post
%systemd_post picoflux.service

%preun
%systemd_preun picoflux.service

%postun
%systemd_postun_with_restart picoflux.service
