%define debug_package %{nil}
%define pkgname {{cpkg_name}}
%define version {{cpkg_version}}
%define bindir {{cpkg_bindir}}
%define etcdir {{cpkg_etcdir}}
%define release {{cpkg_release}}
%define dist {{cpkg_dist}}
%define manage_conf {{cpkg_manage_conf}}
%define binary {{cpkg_binary}}
%define tarball {{cpkg_tarball}}
%define contact {{cpkg_contact}}

Name: %{pkgname}
Version: %{version}
Release: %{release}.%{dist}
Summary: NATS Stream based federation for Prometheus
License: Apache-2.0
URL: https://choria.io
Group: System Tools
Source0: %{tarball}
Packager: %{contact}
BuildRoot: %{_tmppath}/%{pkgname}-%{version}-%{release}-root-%(%{__id_u} -n)
Requires: initscripts

%description
NATS Stream based federation for Prometheus

%prep
%setup -q

%build

%install
rm -rf %{buildroot}
%{__install} -d -m0755  %{buildroot}/etc/sysconfig
%{__install} -d -m0755  %{buildroot}/etc/init.d
%{__install} -d -m0755  %{buildroot}/etc/logrotate.d
%{__install} -d -m0755  %{buildroot}%{bindir}
%{__install} -d -m0755  %{buildroot}%{etcdir}
%{__install} -d -m0755  %{buildroot}/var/log
%{__install} -d -m0756  %{buildroot}/var/run/%{pkgname}
%{__install} -m0755 dist/poller.init %{buildroot}/etc/init.d/%{pkgname}-poller
%{__install} -m0755 dist/receiver.init %{buildroot}/etc/init.d/%{pkgname}-receiver
%{__install} -m0644 dist/poller.sysconfig %{buildroot}/etc/sysconfig/%{pkgname}-poller
%{__install} -m0644 dist/receiver.sysconfig %{buildroot}/etc/sysconfig/%{pkgname}-receiver
%{__install} -m0755 dist/prometheus-streams.logrotate %{buildroot}/etc/logrotate.d/%{pkgname}
%if 0%{?manage_conf} > 0
%{__install} -m0640 dist/prometheus-streams.yaml %{buildroot}%{etcdir}/%{pkgname}.yaml
%endif
%{__install} -m0755 %{binary} %{buildroot}%{bindir}/%{pkgname}
touch %{buildroot}/var/log/%{pkgname}.log

%clean
rm -rf %{buildroot}

%post
/sbin/chkconfig --add %{pkgname}-poller || :
/sbin/chkconfig --add %{pkgname}-receiver || :

%postun
if [ "$1" -ge "1" ]; then
  /sbin/service %{pkgname}-poller condrestart &>/dev/null || :
  /sbin/service %{pkgname}-receiver condrestart &>/dev/null || :
fi

%preun
if [ "$1" = "0" ] ; then
  /sbin/service %{pkgname}-poller stop > /dev/null 2>&1
  /sbin/chkconfig --del %{pkgname}-poller || :
  /sbin/service %{pkgname}-receiver stop > /dev/null 2>&1
  /sbin/chkconfig --del %{pkgname}-receiver || :
fi

%files
%if 0%{?manage_conf} > 0
%attr(640, root, nobody) %config(noreplace)%{etcdir}/prometheus-streams.yaml
%endif
%{bindir}/%{pkgname}
/etc/logrotate.d/%{pkgname}
%attr(755, root, root) /etc/init.d/%{pkgname}-poller
%attr(755, root, root) /etc/init.d/%{pkgname}-receiver
%attr(644, root, root) %config(noreplace)/etc/sysconfig/%{pkgname}-poller
%attr(644, root, root) %config(noreplace)/etc/sysconfig/%{pkgname}-receiver
%attr(755, nobody, nobody)/var/run/%{pkgname}
%attr(644, nobody, nobody)/var/log/%{pkgname}.log

%changelog
* Mon Feb 12 2018 R.I.Pienaar <rip@devco.net>
- Initial Release
