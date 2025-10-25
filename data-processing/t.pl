my $prev = -1;

while (<>) {
    chomp;
    /HV: (\d+),/ or die;
    my $cur = int $1;
    printf("$.: bad %d -> %d\n", $prev, $cur) if ($prev >= 0 && $cur - $prev != 1);
    $prev = $cur;
}
