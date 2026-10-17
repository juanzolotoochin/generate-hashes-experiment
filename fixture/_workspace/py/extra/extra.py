import idna


def ascii_host(name):
    return idna.encode(name).decode("ascii")
