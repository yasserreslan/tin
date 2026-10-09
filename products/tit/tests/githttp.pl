#!/usr/bin/perl
# A git server over smart HTTP for the tests (#805): each request runs `git http-backend` as CGI on the repositories
# under ROOT, one connection at a time. Usage: githttp.pl ROOT PORT
use strict;
use warnings;
use IO::Socket::INET;
use File::Temp qw(tempfile);

my ($root, $port) = @ARGV;
my $server = IO::Socket::INET->new(LocalAddr => "127.0.0.1", LocalPort => $port, Listen => 16, ReuseAddr => 1, Proto => "tcp") or die "listen: $!";
$| = 1;
print "listening on $port\n";
while (my $c = $server->accept) {
	binmode $c;
	my $line = <$c>;
	next unless defined $line;
	$line =~ s/\r?\n$//;
	my ($method, $target) = split / /, $line;
	my %h;
	while (my $l = <$c>) {
		$l =~ s/\r?\n$//;
		last if $l eq "";
		my ($k, $v) = split /:\s*/, $l, 2;
		$h{lc $k} = $v;
	}
	my $body = "";
	if (defined $h{"content-length"}) {
		my $n = $h{"content-length"};
		while (length($body) < $n) {
			my $got = read($c, my $buf, $n - length($body));
			last unless $got;
			$body .= $buf;
		}
	} elsif (defined $h{"transfer-encoding"} && $h{"transfer-encoding"} =~ /chunked/i) {
		while (1) {
			my $size = <$c>;
			$size =~ s/\r?\n$//;
			my $n = hex($size);
			last if $n == 0;
			read($c, my $buf, $n);
			$body .= $buf;
			<$c>;
		}
		<$c>;
	}
	my ($path, $query) = split /\?/, $target, 2;
	my ($fh, $file) = tempfile();
	binmode $fh;
	print $fh $body;
	close $fh;
	local $ENV{GIT_PROJECT_ROOT} = $root;
	local $ENV{GIT_HTTP_EXPORT_ALL} = "1";
	local $ENV{REQUEST_METHOD} = $method;
	local $ENV{PATH_INFO} = $path;
	local $ENV{QUERY_STRING} = $query // "";
	local $ENV{CONTENT_TYPE} = $h{"content-type"} // "";
	local $ENV{CONTENT_LENGTH} = length($body);
	local $ENV{REMOTE_USER} = "tit";
	local $ENV{REMOTE_ADDR} = "127.0.0.1";
	my $out = `git http-backend < '$file'`;
	unlink $file;
	my ($head, $rest) = split /\r?\n\r?\n/, $out, 2;
	$rest //= "";
	my $status = "200 OK";
	my @headers;
	for my $hl (split /\r?\n/, $head // "") {
		if ($hl =~ /^Status:\s*(.*)$/i) {
			$status = $1;
		} else {
			push @headers, $hl;
		}
	}
	print $c "HTTP/1.1 $status\r\n";
	print $c "$_\r\n" for @headers;
	print $c "Content-Length: " . length($rest) . "\r\nConnection: close\r\n\r\n";
	print $c $rest;
	close $c;
}
