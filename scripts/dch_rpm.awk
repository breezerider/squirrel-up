function print_header_dch(package, version, release, urgency) {
    print package " (" version ") " release "; urgency=" urgency "\n";
}

function print_footer_dch(author, email, date) {
    print "\n -- " author " <" email ">  " strftime("%a, %e %b %Y %H:%M:%S %z", date) "\n";
}

function print_header_rpm(author, email, date) {
    print "* " strftime("%a %b %e %Y", date) " " author " <" email ">";
}

function print_line_dch(line) {
    print "  * " line;
}

function print_line_rpm(line) {
    print "- " line;
}

function keepachangelog_date(input) {
    split(input, _tmp_date, "-");
    return mktime(_tmp_date[1] " " _tmp_date[2] " " _tmp_date[3] " 00 00 00");
}

function keepachangelog_version(input) {
    return substr(input, 2, length(input)-2);
}
